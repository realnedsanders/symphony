// Package httpapi serves the web UI and the authoring and ceremony operations.
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"time"

	"github.com/realnedsanders/symphony/src/internal/author"
	"github.com/realnedsanders/symphony/src/internal/ceremony"
	"github.com/realnedsanders/symphony/src/internal/collab"
	"github.com/realnedsanders/symphony/src/internal/model"
	"github.com/realnedsanders/symphony/src/internal/store"
)

// New returns the UI and API handler. dir is the git workspace. web is the
// directory that holds index.html and app.js.
func New(dir, web string) (http.Handler, error) {
	if _, err := store.Open(dir); err != nil {
		return nil, err
	}
	srv := &server{dir: dir, web: web}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", srv.page)
	mux.HandleFunc("GET /app.js", srv.script)
	mux.HandleFunc("GET /api/state", srv.state)
	mux.HandleFunc("POST /api/goal", srv.goal)
	mux.HandleFunc("POST /api/gaps", srv.addGap)
	mux.HandleFunc("POST /api/gaps/priority", srv.priority)
	mux.HandleFunc("POST /api/gaps/decompose", srv.decompose)
	mux.HandleFunc("POST /api/standup", srv.standup)
	mux.HandleFunc("POST /api/retro", srv.retro)
	return mux, nil
}

type server struct {
	dir string
	web string
}

func (s *server) page(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(s.web, "index.html"))
}

func (s *server) script(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(s.web, "app.js"))
}

type stateResponse struct {
	Goal     model.Goal      `json:"goal"`
	Gaps     []model.WorkGap `json:"gaps"`
	Standups []model.Standup `json:"standups"`
	Retros   []model.Retro   `json:"retros"`
}

func (s *server) state(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	goal, err := store.ReadGoal(ctx, s.dir)
	if err != nil {
		writeErr(w, err)
		return
	}
	collabState, err := store.ReadCollab(ctx, s.dir)
	if err != nil {
		writeErr(w, err)
		return
	}
	records, err := store.ReadCeremonies(ctx, s.dir)
	if err != nil {
		writeErr(w, err)
		return
	}
	if collabState.Gaps == nil {
		collabState.Gaps = []model.WorkGap{}
	}
	if records.Standups == nil {
		records.Standups = []model.Standup{}
	}
	if records.Retros == nil {
		records.Retros = []model.Retro{}
	}
	writeJSON(w, http.StatusOK, stateResponse{
		Goal:     goal,
		Gaps:     collabState.Gaps,
		Standups: records.Standups,
		Retros:   records.Retros,
	})
}

type goalRequest struct {
	Part                string `json:"part"`
	Interface           string `json:"interface"`
	Shape               string `json:"shape"`
	Owner               string `json:"owner"`
	ConfigKey           string `json:"configKey"`
	ConfigValue         string `json:"configValue"`
	DecisionName        string `json:"decisionName"`
	DecisionAbout       string `json:"decisionAbout"`
	DecisionChoice      string `json:"decisionChoice"`
	DecisionAlternative string `json:"decisionAlternative"`
	DecisionRationale   string `json:"decisionRationale"`
	SensorName          string `json:"sensorName"`
	SensorObserves      string `json:"sensorObserves"`
	SensorAim           string `json:"sensorAim"`
	SensorOutside       string `json:"sensorOutside"`
}

func (s *server) goal(w http.ResponseWriter, r *http.Request) {
	var req goalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, err)
		return
	}
	if req.Part == "" {
		writeErr(w, errors.New("part is required"))
		return
	}
	goal := model.Goal{
		Parts: []model.Part{{Name: req.Part, Def: "Component"}},
	}
	if req.Owner != "" {
		goal.Owners = append(goal.Owners, model.Ownership{Part: req.Part, Owner: req.Owner})
	}
	if req.ConfigKey != "" {
		goal.Configurations = append(goal.Configurations, model.Configuration{Part: req.Part, Key: req.ConfigKey, Value: req.ConfigValue})
	}
	if req.Interface != "" {
		shape := req.Shape
		if shape == "" {
			shape = "v1"
		}
		goal.Interfaces = append(goal.Interfaces, model.Interface{Name: req.Interface, Shape: shape})
		goal.Places = append(goal.Places, model.Place{Part: req.Part, Port: "input", Interface: req.Interface})
	}
	if req.DecisionName != "" {
		goal.Decisions = append(goal.Decisions, model.Decision{
			Name:        req.DecisionName,
			About:       req.DecisionAbout,
			Choice:      req.DecisionChoice,
			Alternative: req.DecisionAlternative,
			Rationale:   req.DecisionRationale,
			Status:      "rejected",
		})
	}
	if req.SensorName != "" {
		goal.Sensors = append(goal.Sensors, model.SensorContract{
			Name:     req.SensorName,
			Observes: req.SensorObserves,
			Aim:      req.SensorAim,
			Outside:  req.SensorOutside,
		})
	}
	if _, err := author.Apply(r.Context(), s.dir, goal); err != nil {
		writeErr(w, err)
		return
	}
	stored, err := store.ReadGoal(r.Context(), s.dir)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stored)
}

type gapRequest struct {
	ID       string `json:"id"`
	Subject  string `json:"subject"`
	Summary  string `json:"summary"`
	Priority int    `json:"priority"`
}

func (s *server) addGap(w http.ResponseWriter, r *http.Request) {
	var req gapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, err)
		return
	}
	collabState, err := collab.AddGap(r.Context(), s.dir, model.WorkGap{
		ID: req.ID, Subject: req.Subject, Summary: req.Summary, Priority: req.Priority,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, collabState.Gaps)
}

type priorityRequest struct {
	ID       string `json:"id"`
	Priority int    `json:"priority"`
}

func (s *server) priority(w http.ResponseWriter, r *http.Request) {
	var req priorityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, err)
		return
	}
	collabState, err := collab.Prioritize(r.Context(), s.dir, req.ID, req.Priority)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, collabState.Gaps)
}

type decomposeRequest struct {
	ID       string          `json:"id"`
	Children []model.WorkGap `json:"children"`
}

func (s *server) decompose(w http.ResponseWriter, r *http.Request) {
	var req decomposeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, err)
		return
	}
	collabState, err := collab.Decompose(r.Context(), s.dir, req.ID, req.Children)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, collabState.Gaps)
}

type standupRequest struct {
	ID      string `json:"id"`
	Blocker string `json:"blocker"`
}

func (s *server) standup(w http.ResponseWriter, r *http.Request) {
	var req standupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, err)
		return
	}
	if req.ID == "" {
		req.ID = "standup"
	}
	var blockers []string
	if req.Blocker != "" {
		blockers = []string{req.Blocker}
	}
	recorded, err := ceremony.RecordStandup(r.Context(), s.dir, req.ID, time.Time{}, nil, blockers)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, recorded)
}

func (s *server) retro(w http.ResponseWriter, r *http.Request) {
	recorded, err := ceremony.RecordRetro(r.Context(), s.dir, "retro")
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, recorded)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
}
