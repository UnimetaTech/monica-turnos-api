package store

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"strings"

	"monica-turnos-api/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func IsNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (repo *Repository) ListResearchers(ctx context.Context) ([]domain.YoungResearcher, error) {
	rows, err := repo.db.Query(ctx, `
		SELECT id, name, COALESCE(email, ''), active, color, created_at, updated_at
		FROM young_researchers
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	researchers := []domain.YoungResearcher{}

	for rows.Next() {
		var item domain.YoungResearcher

		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Email,
			&item.Active,
			&item.Color,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, err
		}

		researchers = append(researchers, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return researchers, nil
}

func (repo *Repository) ListEvents(ctx context.Context) ([]domain.Event, error) {
	rows, err := repo.db.Query(ctx, `
		SELECT
			id,
			google_calendar_event_id,
			title,
			start_at,
			end_at,
			location,
			modality,
			status,
			required_people,
			source,
			description,
			calendar_synced_at,
			created_by,
			created_at,
			updated_at
		FROM events
		ORDER BY start_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []domain.Event{}

	for rows.Next() {
		item, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}

		events = append(events, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

func (repo *Repository) GetEventByID(ctx context.Context, eventID string) (domain.Event, error) {
	return scanEvent(repo.db.QueryRow(ctx, `
		SELECT
			id,
			google_calendar_event_id,
			title,
			start_at,
			end_at,
			location,
			modality,
			status,
			required_people,
			source,
			description,
			calendar_synced_at,
			created_by,
			created_at,
			updated_at
		FROM events
		WHERE id = $1
	`, eventID))
}

func (repo *Repository) ListAssignments(ctx context.Context) ([]domain.Assignment, error) {
	rows, err := repo.db.Query(ctx, `
		SELECT
			id,
			event_id,
			young_researcher_id,
			status,
			assigned_by,
			source,
			created_at,
			updated_at
		FROM assignments
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	assignments := []domain.Assignment{}

	for rows.Next() {
		item, err := scanAssignment(rows)
		if err != nil {
			return nil, err
		}

		assignments = append(assignments, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return assignments, nil
}

func (repo *Repository) CreateEvent(ctx context.Context, input domain.CreateEventInput) (domain.Event, error) {
	return createEvent(ctx, repo.db, input)
}

func (repo *Repository) CreateEventWithMonicaAssignments(
	ctx context.Context,
	input domain.CreateEventInput,
	monicaAssignments []domain.MonicaAssignmentInput,
) (domain.Event, []domain.Assignment, error) {
	tx, err := repo.db.Begin(ctx)
	if err != nil {
		return domain.Event{}, nil, err
	}
	defer func() {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			log.Printf("error revirtiendo transacción de evento MONICA: %v", rollbackErr)
		}
	}()

	event, err := createEvent(ctx, tx, input)
	if err != nil {
		return domain.Event{}, nil, err
	}

	assignments := make([]domain.Assignment, 0, len(monicaAssignments))
	seenResearchers := map[string]struct{}{}
	assignedBy := "MONICA"

	for _, input := range monicaAssignments {
		name := strings.TrimSpace(input.Name)
		log.Printf("Asignando joven %s al evento %s", input.Name, event.ID)

		if name == "" {
			log.Printf("warning: asignación MONICA sin nombre para evento %s; se omite", event.ID)
			continue
		}

		researcherID, err := findResearcherIDByName(ctx, tx, name)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				log.Printf("warning: joven investigador %q no encontrado para evento %s; se omite asignación MONICA", input.Name, event.ID)
				continue
			}

			return domain.Event{}, nil, err
		}

		if _, alreadySeen := seenResearchers[researcherID]; alreadySeen {
			log.Printf("warning: joven investigador %q repetido en monica_assignments para evento %s; se omite duplicado", input.Name, event.ID)
			continue
		}
		seenResearchers[researcherID] = struct{}{}

		assignment, err := createAssignment(ctx, tx, event.ID, domain.CreateAssignmentInput{
			YoungResearcherID: researcherID,
			Status:            input.Status,
			AssignedBy:        &assignedBy,
			Source:            "MONICA",
		})
		if err != nil {
			return domain.Event{}, nil, err
		}

		assignments = append(assignments, assignment)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Event{}, nil, err
	}

	return event, assignments, nil
}

func createEvent(ctx context.Context, db queryRower, input domain.CreateEventInput) (domain.Event, error) {
	return scanEvent(db.QueryRow(ctx, `
		INSERT INTO events (
			google_calendar_event_id,
			title,
			start_at,
			end_at,
			location,
			modality,
			status,
			required_people,
			source,
			description,
			created_by
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (google_calendar_event_id)
		DO UPDATE SET
			title = EXCLUDED.title,
			start_at = EXCLUDED.start_at,
			end_at = EXCLUDED.end_at,
			location = EXCLUDED.location,
			modality = EXCLUDED.modality,
			status = EXCLUDED.status,
			required_people = EXCLUDED.required_people,
			source = EXCLUDED.source,
			description = EXCLUDED.description,
			updated_at = NOW()
		RETURNING
			id,
			google_calendar_event_id,
			title,
			start_at,
			end_at,
			location,
			modality,
			status,
			required_people,
			source,
			description,
			calendar_synced_at,
			created_by,
			created_at,
			updated_at
	`,
		input.GoogleCalendarEventID,
		input.Title,
		input.StartAt,
		input.EndAt,
		input.Location,
		input.Modality,
		input.Status,
		input.RequiredPeople,
		input.Source,
		input.Description,
		input.CreatedBy,
	))
}

func (repo *Repository) UpdateEvent(ctx context.Context, eventID string, input domain.UpdateEventInput) (domain.Event, error) {
	current, err := repo.GetEventByID(ctx, eventID)
	if err != nil {
		return domain.Event{}, err
	}

	title := current.Title
	startAt := current.StartAt
	endAt := current.EndAt
	location := current.Location
	modality := current.Modality
	status := current.Status
	requiredPeople := current.RequiredPeople
	description := current.Description

	if input.Title != nil {
		title = *input.Title
	}

	if input.StartAt != nil {
		startAt = *input.StartAt
	}

	if input.EndAt != nil {
		endAt = *input.EndAt
	}

	if input.Location != nil {
		location = *input.Location
	}

	if input.Modality != nil {
		modality = *input.Modality
	}

	if input.Status != nil {
		status = *input.Status
	}

	if input.RequiredPeople != nil {
		requiredPeople = *input.RequiredPeople
	}

	if input.Description != nil {
		description = input.Description
	}

	return scanEvent(repo.db.QueryRow(ctx, `
		UPDATE events
		SET
			title = $2,
			start_at = $3,
			end_at = $4,
			location = $5,
			modality = $6,
			status = $7,
			required_people = $8,
			description = $9,
			source = 'PANEL',
			updated_at = NOW()
		WHERE id = $1
		RETURNING
			id,
			google_calendar_event_id,
			title,
			start_at,
			end_at,
			location,
			modality,
			status,
			required_people,
			source,
			description,
			calendar_synced_at,
			created_by,
			created_at,
			updated_at
	`,
		eventID,
		title,
		startAt,
		endAt,
		location,
		modality,
		status,
		requiredPeople,
		description,
	))
}

func (repo *Repository) CreateAssignment(ctx context.Context, eventID string, input domain.CreateAssignmentInput) (domain.Assignment, error) {
	return createAssignment(ctx, repo.db, eventID, input)
}

func createAssignment(ctx context.Context, db queryRower, eventID string, input domain.CreateAssignmentInput) (domain.Assignment, error) {
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "assigned"
	}

	source := strings.TrimSpace(input.Source)
	if source == "" {
		source = "PANEL"
	}

	return scanAssignment(db.QueryRow(ctx, `
		INSERT INTO assignments (
			event_id,
			young_researcher_id,
			status,
			assigned_by,
			source
		)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (event_id, young_researcher_id)
		WHERE status IN ('assigned', 'confirmed')
		DO UPDATE SET
			assigned_by = EXCLUDED.assigned_by,
			source = EXCLUDED.source,
			updated_at = NOW()
		RETURNING
			id,
			event_id,
			young_researcher_id,
			status,
			assigned_by,
			source,
			created_at,
			updated_at
	`,
		eventID,
		input.YoungResearcherID,
		status,
		input.AssignedBy,
		source,
	))
}

func findResearcherIDByName(ctx context.Context, db queryRower, name string) (string, error) {
	var researcherID string

	err := db.QueryRow(ctx, `
		SELECT id
		FROM young_researchers
		WHERE LOWER(TRIM(name)) = LOWER($1)
		ORDER BY created_at ASC
		LIMIT 1
	`, strings.TrimSpace(name)).Scan(&researcherID)

	return researcherID, err
}

func (repo *Repository) DeleteAssignment(ctx context.Context, assignmentID string) (string, error) {
	var eventID string

	err := repo.db.QueryRow(ctx, `
		DELETE FROM assignments
		WHERE id = $1
		RETURNING event_id
	`, assignmentID).Scan(&eventID)

	return eventID, err
}

const researcherAvailabilitySelect = `
	SELECT
		yr.id,
		yr.name,
		COALESCE(yr.email, ''),
		yr.active,
		yr.color,
		CASE
			WHEN NOT yr.active THEN 'inactive'
			WHEN assigned_this.id IS NOT NULL THEN 'assigned_to_this_event'
			WHEN conflicting_event.id IS NOT NULL THEN 'busy'
			ELSE 'available'
		END AS availability,
		CASE
			WHEN NOT yr.active THEN 'Joven investigador inactivo'
			WHEN assigned_this.id IS NOT NULL THEN 'Ya asignado a este evento'
			WHEN conflicting_event.id IS NOT NULL THEN 'Asignado a otro evento en el mismo horario'
			ELSE NULL
		END AS reason,
		conflicting_event.id::text,
		conflicting_event.title,
		conflicting_event.start_at,
		conflicting_event.end_at
	FROM young_researchers yr
	LEFT JOIN LATERAL (
		SELECT a.id
		FROM assignments a
		WHERE a.event_id = $1
			AND a.young_researcher_id = yr.id
			AND a.status IN ('assigned', 'confirmed')
		LIMIT 1
	) assigned_this ON TRUE
	LEFT JOIN LATERAL (
		SELECT e.id, e.title, e.start_at, e.end_at
		FROM assignments a
		JOIN events e ON e.id = a.event_id
		WHERE a.young_researcher_id = yr.id
			AND a.event_id <> $1
			AND a.status IN ('assigned', 'confirmed')
			AND e.status IN ('scheduled', 'rescheduled', 'needs_review')
			AND e.start_at < $3
			AND e.end_at > $2
		ORDER BY e.start_at ASC
		LIMIT 1
	) conflicting_event ON TRUE
`

func (repo *Repository) ListResearcherAvailability(ctx context.Context, targetEvent domain.Event) ([]domain.ResearcherAvailability, error) {
	rows, err := repo.db.Query(
		ctx,
		researcherAvailabilitySelect+`
		ORDER BY yr.name
		`,
		targetEvent.ID,
		targetEvent.StartAt,
		targetEvent.EndAt,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	researchers := []domain.ResearcherAvailability{}

	for rows.Next() {
		item, err := scanResearcherAvailability(rows)
		if err != nil {
			return nil, err
		}

		researchers = append(researchers, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return researchers, nil
}

func (repo *Repository) GetResearcherAvailability(ctx context.Context, targetEvent domain.Event, researcherID string) (domain.ResearcherAvailability, error) {
	return scanResearcherAvailability(repo.db.QueryRow(
		ctx,
		researcherAvailabilitySelect+`
		WHERE yr.id = $4
		LIMIT 1
		`,
		targetEvent.ID,
		targetEvent.StartAt,
		targetEvent.EndAt,
		researcherID,
	))
}

func (repo *Repository) ListEventSyncAssignments(ctx context.Context, eventID string) ([]domain.EventSyncAssignment, error) {
	rows, err := repo.db.Query(ctx, `
		SELECT
			a.id,
			a.young_researcher_id,
			yr.name,
			COALESCE(yr.email, ''),
			a.status
		FROM assignments a
		JOIN young_researchers yr ON yr.id = a.young_researcher_id
		WHERE a.event_id = $1
			AND a.status IN ('assigned', 'confirmed')
		ORDER BY yr.name
	`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	assignments := []domain.EventSyncAssignment{}

	for rows.Next() {
		var item domain.EventSyncAssignment

		if err := rows.Scan(
			&item.ID,
			&item.YoungResearcherID,
			&item.Name,
			&item.Email,
			&item.Status,
		); err != nil {
			return nil, err
		}

		assignments = append(assignments, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return assignments, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEvent(row rowScanner) (domain.Event, error) {
	var item domain.Event

	err := row.Scan(
		&item.ID,
		&item.GoogleCalendarEventID,
		&item.Title,
		&item.StartAt,
		&item.EndAt,
		&item.Location,
		&item.Modality,
		&item.Status,
		&item.RequiredPeople,
		&item.Source,
		&item.Description,
		&item.CalendarSyncedAt,
		&item.CreatedBy,
		&item.CreatedAt,
		&item.UpdatedAt,
	)

	return item, err
}

func scanAssignment(row rowScanner) (domain.Assignment, error) {
	var item domain.Assignment

	err := row.Scan(
		&item.ID,
		&item.EventID,
		&item.YoungResearcherID,
		&item.Status,
		&item.AssignedBy,
		&item.Source,
		&item.CreatedAt,
		&item.UpdatedAt,
	)

	return item, err
}

func scanResearcherAvailability(row rowScanner) (domain.ResearcherAvailability, error) {
	var item domain.ResearcherAvailability
	var reason sql.NullString
	var conflictID sql.NullString
	var conflictTitle sql.NullString
	var conflictStartAt sql.NullTime
	var conflictEndAt sql.NullTime

	if err := row.Scan(
		&item.ID,
		&item.Name,
		&item.Email,
		&item.Active,
		&item.Color,
		&item.Availability,
		&reason,
		&conflictID,
		&conflictTitle,
		&conflictStartAt,
		&conflictEndAt,
	); err != nil {
		return domain.ResearcherAvailability{}, err
	}

	if reason.Valid {
		reasonText := reason.String
		item.Reason = &reasonText
	}

	if conflictID.Valid {
		item.ConflictingEvent = &domain.AvailabilityConflictEvent{
			ID:      conflictID.String,
			Title:   conflictTitle.String,
			StartAt: conflictStartAt.Time,
			EndAt:   conflictEndAt.Time,
		}
	}

	return item, nil
}
