package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"monica-turnos-api/internal/domain"
	"monica-turnos-api/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

type Repository interface {
	ListResearchers(ctx context.Context) ([]domain.YoungResearcher, error)
	ListEvents(ctx context.Context) ([]domain.Event, error)
	GetEventByID(ctx context.Context, eventID string) (domain.Event, error)
	ListAssignments(ctx context.Context) ([]domain.Assignment, error)
	CreateEvent(ctx context.Context, input domain.CreateEventInput) (domain.Event, error)
	CreateEventWithMonicaAssignments(ctx context.Context, input domain.CreateEventInput, assignments []domain.MonicaAssignmentInput) (domain.Event, []domain.Assignment, error)
	UpdateEvent(ctx context.Context, eventID string, input domain.UpdateEventInput) (domain.Event, error)
	CreateAssignment(ctx context.Context, eventID string, input domain.CreateAssignmentInput) (domain.Assignment, error)
	DeleteAssignment(ctx context.Context, assignmentID string) (string, error)
	ListResearcherAvailability(ctx context.Context, targetEvent domain.Event) ([]domain.ResearcherAvailability, error)
	GetResearcherAvailability(ctx context.Context, targetEvent domain.Event, researcherID string) (domain.ResearcherAvailability, error)
}

type Syncer interface {
	TriggerAndLog(ctx context.Context, action string, eventID string, changedBy *string)
}

type API struct {
	repo   Repository
	syncer Syncer
}

type MonicaAssignmentInput struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type CreateEventRequest struct {
	GoogleCalendarEventID *string                 `json:"google_calendar_event_id"`
	Title                 string                  `json:"title"`
	StartAt               string                  `json:"start_at"`
	EndAt                 string                  `json:"end_at"`
	Location              string                  `json:"location"`
	Modality              string                  `json:"modality"`
	Status                string                  `json:"status"`
	RequiredPeople        int                     `json:"required_people"`
	Source                string                  `json:"source"`
	Description           *string                 `json:"description"`
	CreatedBy             *string                 `json:"created_by"`
	MonicaAssignments     []MonicaAssignmentInput `json:"monica_assignments"`
}

type UpdateEventRequest struct {
	Title          *string `json:"title"`
	StartAt        *string `json:"start_at"`
	EndAt          *string `json:"end_at"`
	Location       *string `json:"location"`
	Modality       *string `json:"modality"`
	Status         *string `json:"status"`
	RequiredPeople *int    `json:"required_people"`
	Description    *string `json:"description"`
	ChangedBy      *string `json:"changed_by"`
}

type CreateAssignmentRequest struct {
	YoungResearcherID string  `json:"young_researcher_id"`
	AssignedBy        *string `json:"assigned_by"`
	Source            string  `json:"source"`
}

type CreateEventResponse struct {
	domain.Event
	Assignments []domain.Assignment `json:"assignments"`
}

func NewRouter(repo Repository, syncer Syncer, frontendOrigin string) http.Handler {
	api := &API{
		repo:   repo,
		syncer: syncer,
	}

	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{frontendOrigin},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", api.health)

	r.Route("/api", func(r chi.Router) {
		r.Get("/researchers", api.listResearchers)
		r.Get("/events", api.listEvents)
		r.Post("/events", api.createEvent)
		r.Patch("/events/{id}", api.updateEvent)
		r.Get("/events/{id}/researchers/availability", api.listEventResearcherAvailability)

		r.Get("/assignments", api.listAssignments)
		r.Post("/events/{id}/assignments", api.createAssignment)
		r.Delete("/assignments/{id}", api.deleteAssignment)
	})

	return r
}

func (api *API) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"service": "monica-turnos-api",
	})
}

func (api *API) listResearchers(w http.ResponseWriter, r *http.Request) {
	researchers, err := api.repo.ListResearchers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, researchers)
}

func (api *API) listEvents(w http.ResponseWriter, r *http.Request) {
	events, err := api.repo.ListEvents(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, events)
}

func (api *API) listAssignments(w http.ResponseWriter, r *http.Request) {
	assignments, err := api.repo.ListAssignments(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, assignments)
}

func (api *API) createEvent(w http.ResponseWriter, r *http.Request) {
	var req CreateEventRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	log.Printf("MONICA assignments recibidos: %d", len(req.MonicaAssignments))

	if req.Title == "" || req.StartAt == "" || req.EndAt == "" {
		writeError(w, http.StatusBadRequest, errors.New("title, start_at y end_at son obligatorios"))
		return
	}

	startAt, err := time.Parse(time.RFC3339, req.StartAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("start_at debe venir en formato RFC3339"))
		return
	}

	endAt, err := time.Parse(time.RFC3339, req.EndAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("end_at debe venir en formato RFC3339"))
		return
	}

	if req.Modality == "" {
		req.Modality = "presencial"
	}

	if req.Status == "" {
		req.Status = "scheduled"
	}

	if req.Source == "" {
		req.Source = "PANEL"
	}

	eventInput := domain.CreateEventInput{
		GoogleCalendarEventID: req.GoogleCalendarEventID,
		Title:                 req.Title,
		StartAt:               startAt,
		EndAt:                 endAt,
		Location:              req.Location,
		Modality:              req.Modality,
		Status:                req.Status,
		RequiredPeople:        req.RequiredPeople,
		Source:                req.Source,
		Description:           req.Description,
		CreatedBy:             req.CreatedBy,
	}

	var (
		event             domain.Event
		assignments       []domain.Assignment
		monicaAssignments []domain.MonicaAssignmentInput
	)

	if len(req.MonicaAssignments) > 0 {
		monicaAssignments = make([]domain.MonicaAssignmentInput, 0, len(req.MonicaAssignments))
		for _, input := range req.MonicaAssignments {
			monicaAssignments = append(monicaAssignments, domain.MonicaAssignmentInput{
				Name:   input.Name,
				Status: input.Status,
			})
		}

		event, assignments, err = api.repo.CreateEventWithMonicaAssignments(r.Context(), eventInput, monicaAssignments)
	} else {
		event, err = api.repo.CreateEvent(r.Context(), eventInput)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	api.syncer.TriggerAndLog(context.WithoutCancel(r.Context()), "event_created", event.ID, req.CreatedBy)

	if len(req.MonicaAssignments) > 0 {
		if assignments == nil {
			assignments = []domain.Assignment{}
		}

		writeJSON(w, http.StatusCreated, CreateEventResponse{
			Event:       event,
			Assignments: assignments,
		})
		return
	}

	writeJSON(w, http.StatusCreated, event)
}

func (api *API) updateEvent(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")

	var req UpdateEventRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	input := domain.UpdateEventInput{
		Title:          req.Title,
		Location:       req.Location,
		Modality:       req.Modality,
		Status:         req.Status,
		RequiredPeople: req.RequiredPeople,
		Description:    req.Description,
	}

	if req.StartAt != nil {
		parsed, err := time.Parse(time.RFC3339, *req.StartAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, errors.New("start_at debe venir en formato RFC3339"))
			return
		}
		input.StartAt = &parsed
	}

	if req.EndAt != nil {
		parsed, err := time.Parse(time.RFC3339, *req.EndAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, errors.New("end_at debe venir en formato RFC3339"))
			return
		}
		input.EndAt = &parsed
	}

	updated, err := api.repo.UpdateEvent(r.Context(), eventID, input)
	if err != nil {
		if store.IsNotFound(err) {
			writeError(w, http.StatusNotFound, errors.New("evento no encontrado"))
			return
		}

		writeError(w, http.StatusInternalServerError, err)
		return
	}

	api.syncer.TriggerAndLog(context.WithoutCancel(r.Context()), "event_updated", updated.ID, req.ChangedBy)

	writeJSON(w, http.StatusOK, updated)
}

func (api *API) listEventResearcherAvailability(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")

	targetEvent, err := api.repo.GetEventByID(r.Context(), eventID)
	if err != nil {
		if store.IsNotFound(err) {
			writeError(w, http.StatusNotFound, errors.New("evento no encontrado"))
			return
		}

		writeError(w, http.StatusInternalServerError, err)
		return
	}

	researchers, err := api.repo.ListResearcherAvailability(r.Context(), targetEvent)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, researchers)
}

func (api *API) createAssignment(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")

	var req CreateAssignmentRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if req.YoungResearcherID == "" {
		writeError(w, http.StatusBadRequest, errors.New("young_researcher_id es obligatorio"))
		return
	}

	if req.Source == "" {
		req.Source = "PANEL"
	}

	targetEvent, err := api.repo.GetEventByID(r.Context(), eventID)
	if err != nil {
		if store.IsNotFound(err) {
			writeError(w, http.StatusNotFound, errors.New("evento no encontrado"))
			return
		}

		writeError(w, http.StatusInternalServerError, err)
		return
	}

	availability, err := api.repo.GetResearcherAvailability(r.Context(), targetEvent, req.YoungResearcherID)
	if err != nil {
		if store.IsNotFound(err) {
			writeError(w, http.StatusNotFound, errors.New("joven investigador no encontrado"))
			return
		}

		writeError(w, http.StatusInternalServerError, err)
		return
	}

	switch availability.Availability {
	case "inactive":
		writeError(w, http.StatusConflict, errors.New("El joven investigador está inactivo."))
		return
	case "busy":
		payload := map[string]any{
			"error": "El joven investigador no está disponible en ese horario.",
		}

		if availability.ConflictingEvent != nil {
			payload["conflicting_event"] = availability.ConflictingEvent
		}

		writeJSON(w, http.StatusConflict, payload)
		return
	}

	assignment, err := api.repo.CreateAssignment(r.Context(), eventID, domain.CreateAssignmentInput{
		YoungResearcherID: req.YoungResearcherID,
		AssignedBy:        req.AssignedBy,
		Source:            req.Source,
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			writeError(w, http.StatusConflict, errors.New("El joven investigador ya está asignado a este evento."))
			return
		}

		writeError(w, http.StatusInternalServerError, err)
		return
	}

	api.syncer.TriggerAndLog(context.WithoutCancel(r.Context()), "assignment_created", assignment.EventID, req.AssignedBy)

	writeJSON(w, http.StatusCreated, assignment)
}

func (api *API) deleteAssignment(w http.ResponseWriter, r *http.Request) {
	assignmentID := chi.URLParam(r, "id")

	eventID, err := api.repo.DeleteAssignment(r.Context(), assignmentID)
	if err != nil {
		if store.IsNotFound(err) {
			writeError(w, http.StatusNotFound, errors.New("asignación no encontrada"))
			return
		}

		writeError(w, http.StatusInternalServerError, err)
		return
	}

	changedBy := "PANEL"
	api.syncer.TriggerAndLog(context.WithoutCancel(r.Context()), "assignment_deleted", eventID, &changedBy)

	writeJSON(w, http.StatusOK, map[string]any{
		"deleted": true,
		"id":      assignmentID,
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Println("error escribiendo json:", err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{
		"error": err.Error(),
	})
}
