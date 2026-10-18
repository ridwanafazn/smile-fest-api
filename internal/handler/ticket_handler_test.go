package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ridwanafazn/smile-fest-api/internal/model"
	"github.com/ridwanafazn/smile-fest-api/internal/service"
)

type mockTicketSvc struct {
	service.TicketService
	fnGetVariants func() ([]model.TicketVariant, error)
}

func (m *mockTicketSvc) GetActiveTicketVariants() ([]model.TicketVariant, error) {
	return m.fnGetVariants()
}

func TestGetTicketInfo_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockTicketSvc{fnGetVariants: func() ([]model.TicketVariant, error) {
		return []model.TicketVariant{{ID: "v1", Name: "Presale", Price: 100000, IsActive: true}}, nil
	}}
	h := NewTicketHandler(svc)

	r := gin.New()
	r.GET("/api/tickets/info", h.GetTicketInfo)

	req := httptest.NewRequest(http.MethodGet, "/api/tickets/info", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", w.Code)
	}

	var resp struct {
		Meta struct {
			Code    int    `json:"code"`
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"meta"`
		Data []model.TicketVariant `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if resp.Meta.Code != 200 || resp.Meta.Status != "success" {
		t.Errorf("meta = %+v", resp.Meta)
	}
	if len(resp.Data) != 1 || resp.Data[0].Name != "Presale" {
		t.Errorf("data = %+v", resp.Data)
	}
}

func TestGetTicketInfo_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockTicketSvc{fnGetVariants: func() ([]model.TicketVariant, error) {
		return nil, errors.New("db down")
	}}
	h := NewTicketHandler(svc)

	r := gin.New()
	r.GET("/api/tickets/info", h.GetTicketInfo)

	req := httptest.NewRequest(http.MethodGet, "/api/tickets/info", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", w.Code)
	}
}
