package repo

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	_ "github.com/mattn/go-sqlite3"
)

// 确保 UserWithValueObjects 实现 entity.Entity 接口
var _ entity.Entity = (*UserWithValueObjects)(nil)

// Address 值对象
type Address struct {
	Street  string
	City    string
	ZipCode string
}

// ContactInfo 值对象
type ContactInfo struct {
	Email   string
	Phone   string
	Address Address
}

// UserWithValueObjects 包含值对象的测试实体
type UserWithValueObjects struct {
	ID          string
	Name        string
	ContactInfo ContactInfo
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

// GetID 实现 entity.Entity 接口
func (u *UserWithValueObjects) GetID() string {
	return u.ID
}

// SetID 实现 entity.Entity 接口
func (u *UserWithValueObjects) SetID(id string) {
	u.ID = id
}

// GetCreatedAt 实现 entity.Entity 接口
func (u *UserWithValueObjects) GetCreatedAt() time.Time {
	return u.CreatedAt
}

// SetCreatedAt 实现 entity.Entity 接口
func (u *UserWithValueObjects) SetCreatedAt(t time.Time) {
	u.CreatedAt = t
}

// GetUpdatedAt 实现 entity.Entity 接口
func (u *UserWithValueObjects) GetUpdatedAt() time.Time {
	return u.UpdatedAt
}

// SetUpdatedAt 实现 entity.Entity 接口
func (u *UserWithValueObjects) SetUpdatedAt(t time.Time) {
	u.UpdatedAt = t
}

// GetDeletedAt 实现 entity.Entity 接口
func (u *UserWithValueObjects) GetDeletedAt() *time.Time {
	return u.DeletedAt
}

// SetDeletedAt 实现 entity.Entity 接口
func (u *UserWithValueObjects) SetDeletedAt(t *time.Time) {
	u.DeletedAt = t
}

// TestSqlDBRepository_ValueObjects 测试值对象支持
func TestSqlDBRepository_ValueObjects(t *testing.T) {
	// 创建内存数据库
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// 创建表（包含值对象字段）
	_, err = db.Exec(`
	CREATE TABLE userwithvalueobjects (
		id TEXT PRIMARY KEY,
		name TEXT,
		contact_info_email TEXT,
		contact_info_phone TEXT,
		contact_info_address_street TEXT,
		contact_info_address_city TEXT,
		contact_info_address_zip_code TEXT,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// 创建 Repository
	repo := NewSqlDBRepository[*UserWithValueObjects](db)

	// 创建测试实体
	user := &UserWithValueObjects{
		ID:   "user-1",
		Name: "John Doe",
		ContactInfo: ContactInfo{
			Email: "john@example.com",
			Phone: "123-456-7890",
			Address: Address{
				Street:  "123 Main St",
				City:    "New York",
				ZipCode: "10001",
			},
		},
	}

	// 测试保存
	ctx := context.Background()
	err = repo.Save(ctx, user)
	if err != nil {
		t.Fatalf("Failed to save user: %v", err)
	}

	// 测试读取
	foundUser, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to find user: %v", err)
	}

	// 验证字段
	if foundUser.Name != user.Name {
		t.Errorf("Expected name %s, got %s", user.Name, foundUser.Name)
	}

	if foundUser.ContactInfo.Email != user.ContactInfo.Email {
		t.Errorf("Expected email %s, got %s", user.ContactInfo.Email, foundUser.ContactInfo.Email)
	}

	if foundUser.ContactInfo.Phone != user.ContactInfo.Phone {
		t.Errorf("Expected phone %s, got %s", user.ContactInfo.Phone, foundUser.ContactInfo.Phone)
	}

	if foundUser.ContactInfo.Address.Street != user.ContactInfo.Address.Street {
		t.Errorf("Expected street %s, got %s", user.ContactInfo.Address.Street, foundUser.ContactInfo.Address.Street)
	}

	if foundUser.ContactInfo.Address.City != user.ContactInfo.Address.City {
		t.Errorf("Expected city %s, got %s", user.ContactInfo.Address.City, foundUser.ContactInfo.Address.City)
	}

	if foundUser.ContactInfo.Address.ZipCode != user.ContactInfo.Address.ZipCode {
		t.Errorf("Expected zip code %s, got %s", user.ContactInfo.Address.ZipCode, foundUser.ContactInfo.Address.ZipCode)
	}

	// 测试更新
	foundUser.Name = "Jane Doe"
	foundUser.ContactInfo.Email = "jane@example.com"
	foundUser.ContactInfo.Address.City = "Boston"

	err = repo.Save(ctx, foundUser)
	if err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	// 再次读取验证更新
	updatedUser, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to find updated user: %v", err)
	}

	if updatedUser.Name != "Jane Doe" {
		t.Errorf("Expected updated name Jane Doe, got %s", updatedUser.Name)
	}

	if updatedUser.ContactInfo.Email != "jane@example.com" {
		t.Errorf("Expected updated email jane@example.com, got %s", updatedUser.ContactInfo.Email)
	}

	if updatedUser.ContactInfo.Address.City != "Boston" {
		t.Errorf("Expected updated city Boston, got %s", updatedUser.ContactInfo.Address.City)
	}

	// 测试批量保存
	user2 := &UserWithValueObjects{
		ID:   "user-2",
		Name: "Bob Smith",
		ContactInfo: ContactInfo{
			Email: "bob@example.com",
			Phone: "987-654-3210",
			Address: Address{
				Street:  "456 Oak Ave",
				City:    "Chicago",
				ZipCode: "60601",
			},
		},
	}

	err = repo.SaveBatch(ctx, []*UserWithValueObjects{user2})
	if err != nil {
		t.Fatalf("Failed to save batch: %v", err)
	}

	// 测试查找所有
	allUsers, err := repo.FindAll(ctx)
	if err != nil {
		t.Fatalf("Failed to find all users: %v", err)
	}

	if len(allUsers) != 2 {
		t.Errorf("Expected 2 users, got %d", len(allUsers))
	}

	// 测试 Random 方法
	randomUsers, err := repo.Random(ctx, 1)
	if err != nil {
		t.Fatalf("Failed to get random users: %v", err)
	}

	if len(randomUsers) != 1 {
		t.Errorf("Expected 1 random user, got %d", len(randomUsers))
	}
}
