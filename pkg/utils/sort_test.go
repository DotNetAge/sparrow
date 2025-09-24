package utils

import (
	"reflect"
	"testing"
	"time"

	"github.com/DotNetAge/sparrow/pkg/usecase"
)

// TestSortEntity 测试实体排序功能
// 实现了GetCreatedAt方法的测试实体
type TestSortEntity struct {
	ID        string
	Name      string
	Age       int
	CreatedAt time.Time
}

// GetCreatedAt 返回创建时间
func (e TestSortEntity) GetCreatedAt() time.Time {
	return e.CreatedAt
}

// 用于测试的实体，不实现GetCreatedAt方法
type TestNoTimeEntity struct {
	ID   string
	Name string
	Age  int
}

func TestSortEntities(t *testing.T) {
	// 测试场景1：默认按创建时间降序排序
	now := time.Now()
	twoHoursAgo := now.Add(-2 * time.Hour)
	oneHourAgo := now.Add(-1 * time.Hour)

	entities := []TestSortEntity{
		{ID: "1", Name: "Alice", Age: 30, CreatedAt: twoHoursAgo},
		{ID: "2", Name: "Bob", Age: 25, CreatedAt: now},
		{ID: "3", Name: "Charlie", Age: 35, CreatedAt: oneHourAgo},
	}

	// 预期排序结果：按创建时间降序
	expected := []TestSortEntity{
		{ID: "2", Name: "Bob", Age: 25, CreatedAt: now},
		{ID: "3", Name: "Charlie", Age: 35, CreatedAt: oneHourAgo},
		{ID: "1", Name: "Alice", Age: 30, CreatedAt: twoHoursAgo},
	}

	SortEntities(entities, nil, true)

	// 验证排序结果
	if !reflect.DeepEqual(entities, expected) {
		t.Errorf("Expected sorted by createdAt descending, got %v", entities)
	}

	// 测试场景2：按年龄升序排序
	entities = []TestSortEntity{
		{ID: "1", Name: "Alice", Age: 30, CreatedAt: twoHoursAgo},
		{ID: "2", Name: "Bob", Age: 25, CreatedAt: now},
		{ID: "3", Name: "Charlie", Age: 35, CreatedAt: oneHourAgo},
	}

	expected = []TestSortEntity{
		{ID: "2", Name: "Bob", Age: 25, CreatedAt: now},
		{ID: "1", Name: "Alice", Age: 30, CreatedAt: twoHoursAgo},
		{ID: "3", Name: "Charlie", Age: 35, CreatedAt: oneHourAgo},
	}

	sortFields := []usecase.SortField{
		{Field: "Age", Ascending: true},
	}

	SortEntities(entities, sortFields, true)

	if !reflect.DeepEqual(entities, expected) {
		t.Errorf("Expected sorted by age ascending, got %v", entities)
	}

	// 测试场景3：按名称降序排序
	entities = []TestSortEntity{
		{ID: "1", Name: "Alice", Age: 30, CreatedAt: twoHoursAgo},
		{ID: "2", Name: "Bob", Age: 25, CreatedAt: now},
		{ID: "3", Name: "Charlie", Age: 35, CreatedAt: oneHourAgo},
	}

	expected = []TestSortEntity{
		{ID: "3", Name: "Charlie", Age: 35, CreatedAt: oneHourAgo},
		{ID: "2", Name: "Bob", Age: 25, CreatedAt: now},
		{ID: "1", Name: "Alice", Age: 30, CreatedAt: twoHoursAgo},
	}

	sortFields = []usecase.SortField{
		{Field: "Name", Ascending: false},
	}

	SortEntities(entities, sortFields, true)

	if !reflect.DeepEqual(entities, expected) {
		t.Errorf("Expected sorted by name descending, got %v", entities)
	}

	// 测试场景4：无时间戳实体的排序
	noTimeEntities := []TestNoTimeEntity{
		{ID: "1", Name: "Alice", Age: 30},
		{ID: "2", Name: "Bob", Age: 25},
		{ID: "3", Name: "Charlie", Age: 35},
	}

	expectedNoTime := []TestNoTimeEntity{
		{ID: "2", Name: "Bob", Age: 25},
		{ID: "1", Name: "Alice", Age: 30},
		{ID: "3", Name: "Charlie", Age: 35},
	}

	sortFields = []usecase.SortField{
		{Field: "Age", Ascending: true},
	}

	SortEntities(noTimeEntities, sortFields, false)

	if !reflect.DeepEqual(noTimeEntities, expectedNoTime) {
		t.Errorf("Expected sorted by age ascending for no time entities, got %v", noTimeEntities)
	}
}