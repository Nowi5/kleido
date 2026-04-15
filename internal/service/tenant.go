package service

import (
	"context"

	"kleido/internal/model"
	"kleido/internal/repository"

	"github.com/google/uuid"
)

type TenantService interface {
	Create(ctx context.Context, tenant *model.Tenant) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Tenant, error)
	GetBySlug(ctx context.Context, slug string) (*model.Tenant, error)
	List(ctx context.Context) ([]*model.Tenant, error)
}

type tenantService struct {
	repo repository.TenantRepository
}

func NewTenantService(repo repository.TenantRepository) TenantService {
	return &tenantService{repo: repo}
}

func (s *tenantService) Create(ctx context.Context, tenant *model.Tenant) error {
	return s.repo.Create(ctx, tenant)
}

func (s *tenantService) GetByID(ctx context.Context, id uuid.UUID) (*model.Tenant, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *tenantService) GetBySlug(ctx context.Context, slug string) (*model.Tenant, error) {
	return s.repo.FindBySlug(ctx, slug)
}

func (s *tenantService) List(ctx context.Context) ([]*model.Tenant, error) {
	return s.repo.List(ctx)
}