package domain

import "time"

type YoungResearcher struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Active    bool      `json:"active"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Event struct {
	ID                    string     `json:"id"`
	GoogleCalendarEventID *string    `json:"google_calendar_event_id"`
	Title                 string     `json:"title"`
	StartAt               time.Time  `json:"start_at"`
	EndAt                 time.Time  `json:"end_at"`
	Location              string     `json:"location"`
	Modality              string     `json:"modality"`
	Status                string     `json:"status"`
	RequiredPeople        int        `json:"required_people"`
	Source                string     `json:"source"`
	Description           *string    `json:"description"`
	CalendarSyncedAt      *time.Time `json:"calendar_synced_at"`
	CreatedBy             *string    `json:"created_by"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type Assignment struct {
	ID                string    `json:"id"`
	EventID           string    `json:"event_id"`
	YoungResearcherID string    `json:"young_researcher_id"`
	Status            string    `json:"status"`
	AssignedBy        *string   `json:"assigned_by"`
	Source            string    `json:"source"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type AvailabilityConflictEvent struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	StartAt time.Time `json:"start_at"`
	EndAt   time.Time `json:"end_at"`
}

type ResearcherAvailability struct {
	ID               string                     `json:"id"`
	Name             string                     `json:"name"`
	Email            string                     `json:"email"`
	Active           bool                       `json:"active"`
	Color            string                     `json:"color"`
	Availability     string                     `json:"availability"`
	Reason           *string                    `json:"reason,omitempty"`
	ConflictingEvent *AvailabilityConflictEvent `json:"conflicting_event,omitempty"`
}

type EventSyncAssignment struct {
	ID                string `json:"id"`
	YoungResearcherID string `json:"young_researcher_id"`
	Name              string `json:"name"`
	Email             string `json:"email"`
	Status            string `json:"status"`
}

type MonicaAssignmentInput struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type CreateEventInput struct {
	GoogleCalendarEventID *string
	Title                 string
	StartAt               time.Time
	EndAt                 time.Time
	Location              string
	Modality              string
	Status                string
	RequiredPeople        int
	Source                string
	Description           *string
	CreatedBy             *string
}

type UpdateEventInput struct {
	Title          *string
	StartAt        *time.Time
	EndAt          *time.Time
	Location       *string
	Modality       *string
	Status         *string
	RequiredPeople *int
	Description    *string
}

type CreateAssignmentInput struct {
	YoungResearcherID string
	Status            string
	AssignedBy        *string
	Source            string
}
