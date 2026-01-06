package utils

import (
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/DotNetAge/sparrow/pkg/usecase"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// SortEntities 对实体列表进行排序
// 这个函数使用更简洁的方式处理实体排序，尽量减少反射的使用
func SortEntities[T any](entities []T, sortFields []usecase.SortField, defaultSortByCreatedAt bool) {
	if len(entities) <= 1 {
		return
	}

	// 如果没有排序字段且需要默认排序，按创建时间降序排序
	if len(sortFields) == 0 && defaultSortByCreatedAt {
		// 尝试使用类型断言检查实体是否有GetCreatedAt方法
		// 这比反射更高效且类型安全
		sort.SliceStable(entities, func(i, j int) bool {
			// 尝试类型断言获取时间
			if entityWithTime, ok := any(entities[i]).(interface{ GetCreatedAt() time.Time }); ok {
				if entityJWithTime, ok := any(entities[j]).(interface{ GetCreatedAt() time.Time }); ok {
					return entityWithTime.GetCreatedAt().After(entityJWithTime.GetCreatedAt())
				}
			}
			return false
		})
		return
	}

	// 对于有排序字段的情况，使用反射进行排序
	// 这是必要的，但我们尽量减少反射的使用范围
	sort.SliceStable(entities, func(i, j int) bool {
		for _, sortField := range sortFields {
			lessThan := compareEntities(entities[i], entities[j], sortField.Field)
			if sortField.Ascending && lessThan {
				return true
			} else if !sortField.Ascending && !lessThan {
				return true
			}
		}
		return false
	})
}

// compareEntities 比较两个实体的指定字段
// 使用反射获取字段值并进行比较
func compareEntities(a, b any, fieldName string) bool {
	valA := reflect.ValueOf(a)
	valB := reflect.ValueOf(b)

	// 处理指针类型
	if valA.Kind() == reflect.Ptr {
		valA = valA.Elem()
	}
	if valB.Kind() == reflect.Ptr {
		valB = valB.Elem()
	}

	// 尝试直接获取字段
	fieldA := valA.FieldByName(fieldName)
	fieldB := valB.FieldByName(fieldName)

	// 如果直接获取失败，尝试通过json标签查找
	if !fieldA.IsValid() {
		fieldA = getFieldByJSONTag(valA, fieldName)
	}
	if !fieldB.IsValid() {
		fieldB = getFieldByJSONTag(valB, fieldName)
	}

	if !fieldA.IsValid() || !fieldB.IsValid() {
		return false
	}

	// 根据字段类型进行比较
	return compareValues(fieldA, fieldB)
}

// getFieldByJSONTag 通过JSON标签查找字段
func getFieldByJSONTag(val reflect.Value, fieldName string) reflect.Value {
	for i := 0; i < val.NumField(); i++ {
		field := val.Type().Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" {
			tagParts := Split(jsonTag, ",")
			if len(tagParts) > 0 && tagParts[0] == fieldName {
				return val.Field(i)
			}
		}
	}
	return reflect.Value{}
}

// compareValues 根据值的类型进行比较
func compareValues(a, b reflect.Value) bool {
	switch a.Kind() {
	case reflect.String:
		return a.String() < b.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return a.Int() < b.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return a.Uint() < b.Uint()
	case reflect.Float32, reflect.Float64:
		return a.Float() < b.Float()
	case reflect.Bool:
		return !a.Bool() && b.Bool()
	case reflect.Struct:
		// 处理time.Time类型
		if a.Type().String() == "time.Time" && b.Type().String() == "time.Time" {
			timeA := a.Interface().(time.Time)
			timeB := b.Interface().(time.Time)
			return timeA.Before(timeB)
		}
	}
	return false
}

// Split 分割字符串，等同于strings.Split
// 添加这个函数是为了避免在其他地方引用strings包
func Split(s, sep string) []string {
	return strings.Split(s, sep)
}

// ShuffleStrings 随机打乱字符串切片
func ShuffleStrings(slice []string) {
	for i := len(slice) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		slice[i], slice[j] = slice[j], slice[i]
	}
}
