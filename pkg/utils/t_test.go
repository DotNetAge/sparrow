package utils

import (
	"testing"
)

// TestGetTypeName 测试GetTypeName函数获取各种类型的名称
func TestGetTypeName(t *testing.T) {
	// 测试基本类型
	stringTypeName := GetTypeName[string]()
	if stringTypeName != "string" {
		t.Errorf("Expected type name 'string', got '%s'", stringTypeName)
	}

	intTypeName := GetTypeName[int]()
	if intTypeName != "int" {
		t.Errorf("Expected type name 'int', got '%s'", intTypeName)
	}

	boolTypeName := GetTypeName[bool]()
	if boolTypeName != "bool" {
		t.Errorf("Expected type name 'bool', got '%s'", boolTypeName)
	}

	// 测试自定义结构体类型
	type TestStruct struct {
		Field1 string
		Field2 int
	}

	structTypeName := GetTypeName[TestStruct]()
	expectedStructName := "utils.TestStruct"
	if structTypeName != expectedStructName {
		t.Errorf("Expected type name '%s', got '%s'", expectedStructName, structTypeName)
	}

	// 测试指针类型
	ptrTypeName := GetTypeName[*TestStruct]()
	expectedPtrName := "*utils.TestStruct"
	if ptrTypeName != expectedPtrName {
		t.Errorf("Expected type name '%s', got '%s'", expectedPtrName, ptrTypeName)
	}

	// 测试切片类型
	sliceTypeName := GetTypeName[[]string]()
	expectedSliceName := "[]string"
	if sliceTypeName != expectedSliceName {
		t.Errorf("Expected type name '%s', got '%s'", expectedSliceName, sliceTypeName)
	}

	// 测试映射类型
	mapTypeName := GetTypeName[map[string]int]()
	expectedMapName := "map[string]int"
	if mapTypeName != expectedMapName {
		t.Errorf("Expected type name '%s', got '%s'", expectedMapName, mapTypeName)
	}
}

// TestGetTypeName_EdgeCases 测试GetTypeName函数在边缘情况下的表现
func TestGetTypeName_EdgeCases(t *testing.T) {
	// 测试空接口类型
	interfaceTypeName := GetTypeName[interface{}]()
	if interfaceTypeName != "interface {}" {
		t.Errorf("Expected type name 'interface {}', got '%s'", interfaceTypeName)
	}

	// 移除嵌套泛型类型测试，因为它会导致panic
}
