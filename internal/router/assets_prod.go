//go:build !dev

package router

import "github.com/go-chi/chi/v5"

func registerDevRoutes(_ *chi.Mux) {}
