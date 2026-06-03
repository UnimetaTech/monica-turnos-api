package syncn8n

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"monica-turnos-api/internal/domain"
)

type EventReader interface {
	GetEventByID(ctx context.Context, eventID string) (domain.Event, error)
	ListEventSyncAssignments(ctx context.Context, eventID string) ([]domain.EventSyncAssignment, error)
}

type Syncer struct {
	repo       EventReader
	webhookURL string
	secret     string
	client     *http.Client
}

type eventPayload struct {
	ID                    string    `json:"id"`
	GoogleCalendarEventID *string   `json:"google_calendar_event_id"`
	Title                 string    `json:"title"`
	StartAt               time.Time `json:"start_at"`
	EndAt                 time.Time `json:"end_at"`
	Location              string    `json:"location"`
	Modality              string    `json:"modality"`
	Status                string    `json:"status"`
	RequiredPeople        int       `json:"required_people"`
	Description           *string   `json:"description"`
}

type payload struct {
	Source              string                       `json:"source"`
	Action              string                       `json:"action"`
	ChangedBy           string                       `json:"changed_by"`
	OccurredAt          string                       `json:"occurred_at"`
	Event               eventPayload                 `json:"event"`
	Assignments         []domain.EventSyncAssignment `json:"assignments"`
	CalendarDescription string                       `json:"calendar_description"`
}

func New(repo EventReader, webhookURL string, secret string) *Syncer {
	return &Syncer{
		repo:       repo,
		webhookURL: strings.TrimSpace(webhookURL),
		secret:     strings.TrimSpace(secret),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (syncer *Syncer) Trigger(ctx context.Context, action string, eventID string, changedBy *string) error {
	if syncer.webhookURL == "" {
		log.Printf("[n8n-sync] omitida: N8N_SYNC_WEBHOOK_URL no configurada action=%s event_id=%s", action, eventID)
		return nil
	}

	event, err := syncer.repo.GetEventByID(ctx, eventID)
	if err != nil {
		return fmt.Errorf("consultando evento para sync: %w", err)
	}

	assignments, err := syncer.repo.ListEventSyncAssignments(ctx, eventID)
	if err != nil {
		return fmt.Errorf("consultando asignaciones para sync: %w", err)
	}

	changedByValue := normalizeChangedBy(changedBy)
	body, err := json.Marshal(payload{
		Source:     "MONICA_TURNOS_BACKEND",
		Action:     action,
		ChangedBy:  changedByValue,
		OccurredAt: time.Now().In(monicaTimeLocation()).Format(time.RFC3339),
		Event: eventPayload{
			ID:                    event.ID,
			GoogleCalendarEventID: event.GoogleCalendarEventID,
			Title:                 event.Title,
			StartAt:               event.StartAt,
			EndAt:                 event.EndAt,
			Location:              event.Location,
			Modality:              event.Modality,
			Status:                event.Status,
			RequiredPeople:        event.RequiredPeople,
			Description:           event.Description,
		},
		Assignments:         assignments,
		CalendarDescription: buildCalendarDescription(event, assignments, action, changedByValue),
	})
	if err != nil {
		return fmt.Errorf("serializando payload de sync: %w", err)
	}

	syncCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(syncCtx, http.MethodPost, syncer.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creando request de sync: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if syncer.secret != "" {
		req.Header.Set("X-MONICA-SYNC-SECRET", syncer.secret)
	}

	log.Printf("[n8n-sync] disparando action=%s event_id=%s assignments=%d", action, eventID, len(assignments))

	resp, err := syncer.client.Do(req)
	if err != nil {
		return fmt.Errorf("enviando webhook n8n: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("webhook n8n respondió status=%d", resp.StatusCode)
	}

	log.Printf("[n8n-sync] completada action=%s event_id=%s status=%d", action, eventID, resp.StatusCode)

	return nil
}

func (syncer *Syncer) TriggerAndLog(ctx context.Context, action string, eventID string, changedBy *string) {
	if err := syncer.Trigger(ctx, action, eventID, changedBy); err != nil {
		log.Printf("[n8n-sync] error action=%s event_id=%s: %v", action, eventID, err)
	}
}

func normalizeChangedBy(changedBy *string) string {
	if changedBy == nil {
		return "PANEL"
	}

	value := strings.TrimSpace(*changedBy)
	if value == "" {
		return "PANEL"
	}

	return value
}

func buildCalendarDescription(event domain.Event, assignments []domain.EventSyncAssignment, action string, changedBy string) string {
	var builder strings.Builder
	startAt := event.StartAt.In(monicaTimeLocation())
	endAt := event.EndAt.In(monicaTimeLocation())

	builder.WriteString("INFORMACIÓN DEL EVENTO\n\n")
	builder.WriteString("Título: " + event.Title + "\n")
	builder.WriteString("Fecha: " + startAt.Format("2006-01-02") + "\n")
	builder.WriteString("Horario: " + startAt.Format("15:04") + " - " + endAt.Format("15:04") + "\n")
	builder.WriteString("Lugar: " + event.Location + "\n")
	builder.WriteString("Modalidad: " + event.Modality + "\n")
	builder.WriteString("Estado: " + event.Status + "\n")

	if event.Description != nil && strings.TrimSpace(*event.Description) != "" {
		builder.WriteString("Descripción: " + strings.TrimSpace(*event.Description) + "\n")
	}

	builder.WriteString("\nJÓVENES INVESTIGADORES ASIGNADOS\n\n")

	if len(assignments) == 0 {
		builder.WriteString("Sin jóvenes investigadores asignados.\n")
	} else {
		for _, assignment := range assignments {
			builder.WriteString("- " + assignment.Name + "\n")
		}
	}

	builder.WriteString("\nGESTIÓN\n\n")
	builder.WriteString("Actualizado desde MONICA Turnos Panel.\n")
	builder.WriteString("Última acción: " + action + "\n")
	builder.WriteString("Actualizado por: " + changedBy + "\n")

	return builder.String()
}

func monicaTimeLocation() *time.Location {
	return time.FixedZone("America/Bogota", -5*60*60)
}
