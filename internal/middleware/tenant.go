package middleware

import (
	"context"
	"net/http"
	"strings"

	"kleido/internal/model"
	"kleido/internal/reqctx"
	"kleido/internal/service"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type TenantResolver interface {
	GetBySlug(ctx context.Context, slug string) (*model.Tenant, error)
	List(ctx context.Context) ([]*model.Tenant, error)
}

func TenantMiddleware(tenantSvc service.TenantService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), "tenantService", tenantSvc)

			var tenantID uuid.UUID

			if slug := extractSubdomain(r); slug != "" {
				tenant, err := tenantSvc.GetBySlug(ctx, slug)
				if err == nil && tenant != nil {
					tenantID = tenant.ID
				}
			}

			if tenantID == uuid.Nil {
				if idStr := r.URL.Query().Get("tenant_id"); idStr != "" {
					if id, err := uuid.Parse(idStr); err == nil {
						tenantID = id
					}
				}
			}

			if tenantID != uuid.Nil {
				ctx = reqctx.WithTenantID(ctx, tenantID)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractSubdomain(r *http.Request) string {
	host := r.Host
	parts := strings.Split(host, ".")
	if len(parts) >= 3 {
		subdomain := parts[0]
		if subdomain == "" {
			return ""
		}
		isIP := true
		for _, c := range subdomain {
			if c != '.' && (c < '0' || c > '9') {
				isIP = false
				break
			}
		}
		if isIP && strings.Contains(host, ".") {
			return ""
		}
		return subdomain
	}
	return ""
}

func ExtractSubdomain(r *http.Request) string {
	return extractSubdomain(r)
}

func RequireTenantID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := reqctx.TenantIDFromContext(r.Context())
		if tenantID == uuid.Nil {
			http.Error(w, "tenant_id is required", http.StatusBadRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func TenantFromPath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "tenant")
		if slug == "" {
			next.ServeHTTP(w, r)
			return
		}

		ctx := r.Context()
		tenantSvc := getTenantService(ctx)
		if tenantSvc != nil {
			if tenant, err := tenantSvc.GetBySlug(ctx, slug); err == nil && tenant != nil {
				ctx = reqctx.WithTenantID(ctx, tenant.ID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func getTenantService(ctx context.Context) service.TenantService {
	if v := ctx.Value("tenantService"); v != nil {
		if svc, ok := v.(service.TenantService); ok {
			return svc
		}
	}
	return nil
}