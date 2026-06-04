package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"monica-turnos-api/internal/domain"
)

func TestCreateEventKeepsLegacyResponseWithoutMonicaAssignments(t *testing.T) {
	event := testEvent()
	repo := &fakeRepository{event: event}
	router := NewRouter(repo, noopSyncer{}, "*")

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewBufferString(`{
		"title": "Demo",
		"start_at": "2026-06-05T10:00:00Z",
		"end_at": "2026-06-05T11:00:00Z"
	}`))

	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d. body: %s", response.Code, http.StatusCreated, response.Body.String())
	}

	if repo.createEventCalls != 1 {
		t.Fatalf("CreateEvent calls = %d, want 1", repo.createEventCalls)
	}

	if repo.createEventWithMonicaAssignmentsCalls != 0 {
		t.Fatalf("CreateEventWithMonicaAssignments calls = %d, want 0", repo.createEventWithMonicaAssignmentsCalls)
	}

	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if _, ok := payload["assignments"]; ok {
		t.Fatalf("legacy response should not include assignments when monica_assignments is absent: %#v", payload["assignments"])
	}
}

func TestCreateEventReturnsAssignmentsWhenMonicaAssignmentsAreProvided(t *testing.T) {
	event := testEvent()
	assignedBy := "MONICA"
	repo := &fakeRepository{
		event: event,
		assignments: []domain.Assignment{
			{
				ID:                "assignment-1",
				EventID:           event.ID,
				YoungResearcherID: "researcher-1",
				Status:            "assigned",
				AssignedBy:        &assignedBy,
				Source:            "MONICA",
				CreatedAt:         event.CreatedAt,
				UpdatedAt:         event.UpdatedAt,
			},
		},
	}
	router := NewRouter(repo, noopSyncer{}, "*")

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/events", bytes.NewBufferString(`{
		"title": "Demo",
		"start_at": "2026-06-05T10:00:00Z",
		"end_at": "2026-06-05T11:00:00Z",
		"monica_assignments": [
			{ "name": "Sebastian", "status": "assigned" }
		]
	}`))

	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d. body: %s", response.Code, http.StatusCreated, response.Body.String())
	}

	if repo.createEventCalls != 0 {
		t.Fatalf("CreateEvent calls = %d, want 0", repo.createEventCalls)
	}

	if repo.createEventWithMonicaAssignmentsCalls != 1 {
		t.Fatalf("CreateEventWithMonicaAssignments calls = %d, want 1", repo.createEventWithMonicaAssignmentsCalls)
	}

	if len(repo.monicaAssignments) != 1 {
		t.Fatalf("monica assignments passed = %d, want 1", len(repo.monicaAssignments))
	}

	if repo.monicaAssignments[0].Name != "Sebastian" || repo.monicaAssignments[0].Status != "assigned" {
		t.Fatalf("monica assignment = %#v, want Sebastian/assigned", repo.monicaAssignments[0])
	}

	var payload struct {
		ID          string              `json:"id"`
		Assignments []domain.Assignment `json:"assignments"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if payload.ID != event.ID {
		t.Fatalf("event id = %q, want %q", payload.ID, event.ID)
	}

	if len(payload.Assignments) != 1 {
		t.Fatalf("assignments in response = %d, want 1", len(payload.Assignments))
	}

	if payload.Assignments[0].AssignedBy == nil || *payload.Assignments[0].AssignedBy != "MONICA" {
		t.Fatalf("assigned_by = %#v, want MONICA", payload.Assignments[0].AssignedBy)
	}
}

func testEvent() domain.Event {
	startAt := time.Date(2026, 6, 5, 10, 0, 0, 0, time.UTC)
	endAt := startAt.Add(time.Hour)

	return domain.Event{
		ID:             "event-1",
		Title:          "Demo",
		StartAt:        startAt,
		EndAt:          endAt,
		Location:       "",
		Modality:       "presencial",
		Status:         "scheduled",
		RequiredPeople: 0,
		Source:         "PANEL",
		CreatedAt:      startAt,
		UpdatedAt:      startAt,
	}
}

type fakeRepository struct {
	event                                 domain.Event
	assignments                           []domain.Assignment
	monicaAssignments                     []domain.MonicaAssignmentInput
	createEventCalls                      int
	createEventWithMonicaAssignmentsCalls int
}

func (repo *fakeRepository) ListResearchers(ctx context.Context) ([]domain.YoungResearcher, error) {
	return nil, nil
}

func (repo *fakeRepository) ListEvents(ctx context.Context) ([]domain.Event, error) {
	return nil, nil
}

func (repo *fakeRepository) GetEventByID(ctx context.Context, eventID string) (domain.Event, error) {
	return repo.event, nil
}

func (repo *fakeRepository) ListAssignments(ctx context.Context) ([]domain.Assignment, error) {
	return nil, nil
}

func (repo *fakeRepository) CreateEvent(ctx context.Context, input domain.CreateEventInput) (domain.Event, error) {
	repo.createEventCalls++
	return repo.event, nil
}

func (repo *fakeRepository) CreateEventWithMonicaAssignments(
	ctx context.Context,
	input domain.CreateEventInput,
	assignments []domain.MonicaAssignmentInput,
) (domain.Event, []domain.Assignment, error) {
	repo.createEventWithMonicaAssignmentsCalls++
	repo.monicaAssignments = append([]domain.MonicaAssignmentInput(nil), assignments...)

	return repo.event, repo.assignments, nil
}

func (repo *fakeRepository) UpdateEvent(ctx context.Context, eventID string, input domain.UpdateEventInput) (domain.Event, error) {
	return repo.event, nil
}

func (repo *fakeRepository) CreateAssignment(ctx context.Context, eventID string, input domain.CreateAssignmentInput) (domain.Assignment, error) {
	return domain.Assignment{}, nil
}

func (repo *fakeRepository) DeleteAssignment(ctx context.Context, assignmentID string) (string, error) {
	return "", nil
}

func (repo *fakeRepository) ListResearcherAvailability(ctx context.Context, targetEvent domain.Event) ([]domain.ResearcherAvailability, error) {
	return nil, nil
}

func (repo *fakeRepository) GetResearcherAvailability(ctx context.Context, targetEvent domain.Event, researcherID string) (domain.ResearcherAvailability, error) {
	return domain.ResearcherAvailability{}, nil
}

type noopSyncer struct{}

func (noopSyncer) TriggerAndLog(ctx context.Context, action string, eventID string, changedBy *string) {
}
