package utils

import (
	"reflect"
	"strings"
)

// GetTypeName 获取泛型类型 T 的名称
func GetTypeName[T any]() string {
	// 使用reflect.TypeFor直接获取泛型类型信息，避免创建零值可能带来的问题
	return reflect.TypeFor[T]().String()
}

func GetShotTypeName[T any]() string {
	fullName := GetTypeName[T]()
	shortName := fullName[strings.LastIndex(fullName, ".")+1:]
	return shortName
}
