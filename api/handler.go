// Package api provides an http.Handler with handlers for uploading and retrieving
// Diagnosis Keys.
package api

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/dstotijn/ct-diag-server/diag"

	"go.uber.org/zap"
)

type handler struct {
	diagSvc diag.Service
	logger  *zap.Logger
}

// NewHandler returns a new Handler.
func NewHandler(ctx context.Context, cfg diag.Config, logger *zap.Logger) (http.Handler, error) {
	diagSvc, err := diag.NewService(ctx, cfg)
	if err != nil {
		return nil, err
	}

	h := handler{
		diagSvc: diagSvc,
		logger:  logger,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/diagnosis-keys", h.diagnosisKeys)
	mux.HandleFunc("/health", h.health)

	return mux, nil
}

// diagnosisKeys handles both GET and POST requests.
func (h *handler) diagnosisKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listDiagnosisKeys(w, r)
	case http.MethodPost:
		h.postDiagnosisKeys(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// listDiagnosisKeys writes all diagnosis keys as binary data in the HTTP response.
func (h *handler) listDiagnosisKeys(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=0, s-maxage=600")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	itemCount := h.diagSvc.ItemCount()
	w.Header().Set("Content-Length", strconv.Itoa(itemCount*diag.DiagnosisKeySize))

	bw := bufio.NewWriter(w)
	if _, err := h.diagSvc.WriteDiagnosisKeys(bw); err != nil {
		return
	}
	bw.Flush()
}

// postDiagnosisKeys reads POST data from an HTTP request and stores it.
func (h *handler) postDiagnosisKeys(w http.ResponseWriter, r *http.Request) {
	uploadLimit := h.diagSvc.MaxUploadBatchSize() * diag.DiagnosisKeySize
	maxBytesReader := http.MaxBytesReader(w, r.Body, int64(uploadLimit))
	diagKeys, err := diag.ParseDiagnosisKeys(maxBytesReader)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid body: %v", err), http.StatusBadRequest)
		return
	}

	err = h.diagSvc.StoreDiagnosisKeys(r.Context(), diagKeys)
	if err != nil {
		h.logger.Error("Could not store diagnosis keys", zap.Error(err))
		writeInternalErrorResp(w, err)
		return
	}

	fmt.Fprint(w, "OK")
}

// health writes OK in the HTTP response.
func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "OK")
}

func writeInternalErrorResp(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	http.Error(w, http.StatusText(code), code)
}
