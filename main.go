package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"
)


var logger *slog.Logger


func initLogger() {
	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})).With(
		slog.String("env", "local"),
		slog.String("version", "0.0.0"),
	)
}

type LoggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int64
}


func (lrw *LoggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}


func (lrw *LoggingResponseWriter) Write(b []byte) (int, error) {
	if lrw.statusCode == 0 {
		lrw.WriteHeader(http.StatusOK) 
	}
	n, err := lrw.ResponseWriter.Write(b)
	lrw.written += int64(n)
	return n, err
}



func loggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now() 
	
		lrw := &LoggingResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		
		next.ServeHTTP(lrw, r)

	
		logger.Info("request completed",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", lrw.statusCode),
			slog.Duration("duration", time.Since(start)),
			slog.String("remote_addr", r.RemoteAddr),
		)
	})
}

func handleCreateComment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := strconv.Atoi(id); err != nil {
		logger.Warn("некорректный id в URL", "bad_id", id, "error", err)
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	const maxBodySize = 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var input struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() 

	if err := dec.Decode(&input); err != nil {
		logger.Warn("ошибка парсинга входящего JSON", "error", err)
		http.Error(w, "invalid json body or unknown fields", http.StatusBadRequest)
		return
	}


	logger.Debug("данные комментария успешно получены", "name", input.Name, "email", input.Email)
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(`{"status":"success"}`))
}



func main() {

	initLogger()

	mux := http.NewServeMux()
	
	mux.HandleFunc("POST /posts/{id}/comments", handleCreateComment)

	wrappedMux := loggingMiddleware(logger, mux)


	srv := &http.Server{
		Addr:           ":8080",
		Handler:        wrappedMux,       
		ReadTimeout:    5 * time.Second,  
		WriteTimeout:   10 * time.Second, 
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20,          
	}

	logger.Info("запуск сервера", "addr", srv.Addr)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("критическая ошибка сервера", "error", err)
		os.Exit(1)
	}
}
