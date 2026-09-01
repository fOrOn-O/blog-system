package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"blog-system/internal/config"
	"blog-system/internal/database"
	"blog-system/internal/model"
	"blog-system/pkg/auth"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupUserServiceTest(t *testing.T) (*UserService, model.User, model.User, model.User) {
	t.Helper()

	databaseName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", databaseName)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	database.DB = db
	previousConfig := config.AppConfig
	config.AppConfig = &config.Config{JWT: config.JWT{Secret: "user-service-test-secret", Expiration: 1}}
	t.Cleanup(func() { config.AppConfig = previousConfig })

	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	oldPasswordHash, err := auth.HashPassword("OldPassword123")
	if err != nil {
		t.Fatalf("hash test password: %v", err)
	}

	admin := model.User{Username: "admin", Email: "admin@example.com", Password: oldPasswordHash, Role: "admin", IsActive: true}
	otherAdmin := model.User{Username: "other-admin", Email: "other-admin@example.com", Password: oldPasswordHash, Role: "admin", IsActive: true}
	normalUser := model.User{Username: "member", Email: "member@example.com", Password: oldPasswordHash, Role: "user", IsActive: true}
	for _, user := range []*model.User{&admin, &otherAdmin, &normalUser} {
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("create user %q: %v", user.Username, err)
		}
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	return NewUserService(), admin, otherAdmin, normalUser
}

func TestUserServiceUpdateUserStatus(t *testing.T) {
	userService, admin, otherAdmin, normalUser := setupUserServiceTest(t)

	updated, err := userService.UpdateUserStatus(admin.ID, normalUser.ID, false)
	if err != nil {
		t.Fatalf("disable normal user: %v", err)
	}
	if updated.IsActive {
		t.Fatal("expected user to be disabled")
	}

	storedUser, err := userService.userRepo.FindByID(normalUser.ID)
	if err != nil {
		t.Fatalf("reload disabled user: %v", err)
	}
	if storedUser.IsActive {
		t.Fatal("disabled status was not persisted")
	}
	if _, err := NewAuthService().Login(LoginRequest{Username: normalUser.Username, Password: "OldPassword123"}); err == nil {
		t.Fatal("disabled user was still able to log in")
	}

	updated, err = userService.UpdateUserStatus(admin.ID, normalUser.ID, true)
	if err != nil {
		t.Fatalf("enable normal user: %v", err)
	}
	if !updated.IsActive {
		t.Fatal("expected user to be enabled")
	}
	if _, err := NewAuthService().Login(LoginRequest{Username: normalUser.Username, Password: "OldPassword123"}); err != nil {
		t.Fatalf("enabled user could not log in: %v", err)
	}

	if _, err := userService.UpdateUserStatus(admin.ID, admin.ID, false); !errors.Is(err, ErrCannotDisableSelf) {
		t.Fatalf("expected self-disable rejection, got %v", err)
	}
	if _, err := userService.UpdateUserStatus(admin.ID, otherAdmin.ID, false); !errors.Is(err, ErrCannotManageAdmin) {
		t.Fatalf("expected admin status change rejection, got %v", err)
	}
	if _, err := userService.UpdateUserStatus(admin.ID, 999999, false); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected missing user error, got %v", err)
	}
}

func TestUserServiceResetUserPassword(t *testing.T) {
	userService, _, otherAdmin, normalUser := setupUserServiceTest(t)

	if err := userService.ResetUserPassword(normalUser.ID, "NewStrongPassword123"); err != nil {
		t.Fatalf("reset normal user password: %v", err)
	}

	storedUser, err := userService.userRepo.FindByID(normalUser.ID)
	if err != nil {
		t.Fatalf("reload user after password reset: %v", err)
	}
	if storedUser.Password == "NewStrongPassword123" {
		t.Fatal("password was stored in plaintext")
	}
	if !auth.CheckPassword("NewStrongPassword123", storedUser.Password) {
		t.Fatal("new password does not match stored hash")
	}
	if auth.CheckPassword("OldPassword123", storedUser.Password) {
		t.Fatal("old password still matches after reset")
	}

	authService := NewAuthService()
	if _, err := authService.Login(LoginRequest{Username: normalUser.Username, Password: "NewStrongPassword123"}); err != nil {
		t.Fatalf("new password could not be used to log in: %v", err)
	}
	if _, err := authService.Login(LoginRequest{Username: normalUser.Username, Password: "OldPassword123"}); err == nil {
		t.Fatal("old password still allowed login after reset")
	}

	if err := userService.ResetUserPassword(otherAdmin.ID, "AnotherPassword123"); !errors.Is(err, ErrCannotManageAdmin) {
		t.Fatalf("expected admin password reset rejection, got %v", err)
	}
	if err := userService.ResetUserPassword(999999, "AnotherPassword123"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected missing user error, got %v", err)
	}
}

func TestUserServiceListUsersDoesNotExposePasswords(t *testing.T) {
	userService, _, _, _ := setupUserServiceTest(t)

	users, total, err := userService.ListUsers(1, 10)
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if total != 3 || len(users) != 3 {
		t.Fatalf("unexpected users response: total=%d users=%#v", total, users)
	}
	for _, user := range users {
		if user.CreatedAt.IsZero() {
			t.Fatalf("user %d is missing created_at", user.ID)
		}
	}

	encoded, err := json.Marshal(users)
	if err != nil {
		t.Fatalf("marshal users: %v", err)
	}
	if strings.Contains(strings.ToLower(string(encoded)), "password") {
		t.Fatalf("user list exposed a password field: %s", encoded)
	}
}
