// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build cgo
// +build cgo

package artifactcache

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/mattn/go-sqlite3"
	"xorm.io/xorm"
)

func StartHandler(dir, outboundIP string, port uint16) (*Handler, error) {
	h := &Handler{}

	if dir == "" {
		if home, err := os.UserHomeDir(); err != nil {
			return nil, err
		} else {
			dir = filepath.Join(home, ".cache", "actcache")
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	e, err := xorm.NewEngine("sqlite3", filepath.Join(dir, "sqlite.db"))
	if err != nil {
		return nil, err
	}
	if err := e.Sync(&Cache{}); err != nil {
		return nil, err
	}
	h.engine = engine{e: e}

	storage, err := NewStorage(filepath.Join(dir, "cache"))
	if err != nil {
		return nil, err
	}
	h.storage = storage

	if outboundIP != "" {
		h.outboundIP = outboundIP
	} else if ip, err := getOutboundIP(); err != nil {
		return nil, err
	} else {
		h.outboundIP = ip.String()
	}

	router := chi.NewRouter()
	router.Use(middleware.RequestLogger(&middleware.DefaultLogFormatter{Logger: logger}))
	router.Use(func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handler.ServeHTTP(w, r)
			go h.gcCache()
		})
	})
	router.Use(middleware.Logger)
	router.Route(urlBase, func(r chi.Router) {
		r.Get("/cache", h.find)
		r.Route("/caches", func(r chi.Router) {
			r.Post("/", h.reserve)
			r.Route("/{id}", func(r chi.Router) {
				r.Patch("/", h.upload)
				r.Post("/", h.commit)
			})
		})
		r.Get("/artifacts/{id}", h.get)
		r.Post("/clean", h.clean)
	})

	h.router = router

	h.gcCache()

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port)) // listen on all interfaces
	if err != nil {
		return nil, err
	}
	go func() {
		if err := http.Serve(listener, h.router); err != nil {
			logger.Errorf("http serve: %v", err)
		}
	}()
	h.listener = listener

	return h, nil
}
