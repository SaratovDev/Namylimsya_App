package services

import (
	"context"

	"github.com/SaratovDev/Namylimsya_App/api/internal/db/dbgen"
	"github.com/google/uuid"
)

type AdminService struct {
	db *dbgen.Queries
}

func NewAdminService(db *dbgen.Queries) *AdminService {
	return &AdminService{db: db}
}

type Dashboard struct {
	Users         dbgen.AdminDashboardUsersRow        `json:"users"`
	Events        dbgen.AdminDashboardEventsRow       `json:"events"`
	Applications  dbgen.AdminDashboardApplicationsRow `json:"applications"`
	TopCategories []dbgen.AdminTopCategoriesRow       `json:"top_categories"`
	TopOrganizers []dbgen.AdminTopOrganizersRow       `json:"top_organizers"`
	Activity      []dbgen.AdminActivityChartRow       `json:"activity"`
}

func (s *AdminService) GetDashboard(ctx context.Context) (*Dashboard, error) {
	users, err := s.db.AdminDashboardUsers(ctx)
	if err != nil {
		return nil, err
	}
	events, err := s.db.AdminDashboardEvents(ctx)
	if err != nil {
		return nil, err
	}
	apps, err := s.db.AdminDashboardApplications(ctx)
	if err != nil {
		return nil, err
	}
	cats, err := s.db.AdminTopCategories(ctx)
	if err != nil {
		return nil, err
	}
	orgs, err := s.db.AdminTopOrganizers(ctx)
	if err != nil {
		return nil, err
	}
	act, err := s.db.AdminActivityChart(ctx)
	if err != nil {
		return nil, err
	}

	return &Dashboard{
		Users:         users,
		Events:        events,
		Applications:  apps,
		TopCategories: cats,
		TopOrganizers: orgs,
		Activity:      act,
	}, nil
}

func (s *AdminService) ListUsers(ctx context.Context, search *string, role *string, limit, offset int32) ([]dbgen.AdminListUsersRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.db.AdminListUsers(ctx, dbgen.AdminListUsersParams{
		Search: search, Role: role, Limit: limit, Offset: offset,
	})
}

func (s *AdminService) UpdateUserRole(ctx context.Context, userID uuid.UUID, role string) (*dbgen.AdminUpdateUserRoleRow, error) {
	u, err := s.db.AdminUpdateUserRole(ctx, dbgen.AdminUpdateUserRoleParams{
		ID: userID, Role: role,
	})
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *AdminService) BanUser(ctx context.Context, userID uuid.UUID, banned bool, reason string) (*dbgen.AdminBanUserRow, error) {
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	u, err := s.db.AdminBanUser(ctx, dbgen.AdminBanUserParams{
		ID: userID, IsBanned: banned, BanReason: reasonPtr,
	})
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *AdminService) ListEvents(ctx context.Context, status *string, limit, offset int32) ([]dbgen.AdminListEventsRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.db.AdminListEvents(ctx, dbgen.AdminListEventsParams{
		Status: status, Limit: limit, Offset: offset,
	})
}

func (s *AdminService) DeleteEvent(ctx context.Context, id uuid.UUID) error {
	return s.db.AdminDeleteEvent(ctx, id)
}
