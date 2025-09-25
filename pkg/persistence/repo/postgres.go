package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/errs"
	"github.com/DotNetAge/sparrow/pkg/logger"
	"github.com/DotNetAge/sparrow/pkg/usecase"
	"github.com/jmoiron/sqlx"
)

// PostgresRepository PostgreSQL仓储实现
// 基于BaseRepository[T]的完整PostgreSQL实现
// 支持泛型实体类型，提供完整的CRUD操作
type PostgresRepository[T entity.Entity] struct {
	usecase.BaseRepository[T]
	db         *sqlx.DB
	tableName  string
	entityType string
	logger     *logger.Logger
}

var _ usecase.Repository[entity.Entity] = (*PostgresRepository[entity.Entity])(nil)

// 辅助方法：处理多级指针类型，返回解引用后的值和类型
// 特别处理两种场景：
// 1. buildUpdateData中处理实体对象指针
// 2. scanEntity中处理新创建的变量指针（可能是指向nil指针的指针）
func (r *PostgresRepository[T]) dereferencePointer(value reflect.Value) (reflect.Value, reflect.Type, error) {
	valueType := value.Type()

	// 循环处理所有级别的指针类型
	for valueType.Kind() == reflect.Ptr {
		// 对于指向指针的指针，如果内部指针是nil，我们就不继续解引用了
		// 这是为了处理FindWithConditions中创建的新变量指针
		if value.IsZero() {
			return value, valueType, nil
		}

		// 解引用指针
		value = value.Elem()
		valueType = value.Type()
	}

	return value, valueType, nil
}

// NewPostgresRepository 创建PostgreSQL仓储实例
// 参数:
//   - db: PostgreSQL数据库连接
//   - tableName: 数据库表名
//   - logger: 日志记录器
//
// 返回: 初始化的PostgreSQL仓储实例
func NewPostgresRepository[T entity.Entity](db *sqlx.DB, tableName string, logger *logger.Logger) usecase.Repository[T] {
	var zero T
	entityType := fmt.Sprintf("%T", zero)

	return &PostgresRepository[T]{
		BaseRepository: usecase.BaseRepository[T]{},
		db:             db,
		tableName:      tableName,
		entityType:     entityType,
		logger:         logger,
	}
}

// SaveBatch 批量保存实体
func (r *PostgresRepository[T]) SaveBatch(ctx context.Context, entities []T) error {
	if len(entities) == 0 {
		return nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			if r.logger != nil {
				r.logger.Error("Failed to rollback transaction", "error", err)
			}
		}
	}()

	for _, entity := range entities {
		if entity.GetID() == "" {
			if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
				if r.logger != nil {
					r.logger.Error("Failed to rollback transaction", "error", err)
				}
			}
			return &errs.RepositoryError{
				EntityType: r.entityType,
				Operation:  "save_batch",
				Message:    "entity ID cannot be empty",
			}
		}

		exists, err := r.existsInTx(ctx, tx, entity.GetID())
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
				if r.logger != nil {
					r.logger.Error("Failed to rollback transaction", "error", rollbackErr)
				}
			}
			return err
		}

		if exists {
			if err := r.updateInTx(ctx, tx, entity); err != nil {
				if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
					if r.logger != nil {
						r.logger.Error("Failed to rollback transaction", "error", rollbackErr)
					}
				}
				return err
			}
		} else {
			if err := r.insertInTx(ctx, tx, entity); err != nil {
				if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
					if r.logger != nil {
						r.logger.Error("Failed to rollback transaction", "error", rollbackErr)
					}
				}
				return err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Save 保存实体（插入或更新）
// 如果实体ID已存在则执行更新，否则执行插入
func (r *PostgresRepository[T]) Save(ctx context.Context, entity T) error {
	if entity.GetID() == "" {
		return errs.NewRepositoryInvalidError(r.entityType, "save", "entity ID cannot be empty")
	}

	// 检查实体是否存在
	exists, err := r.Exists(ctx, entity.GetID())
	if err != nil {
		return err
	}

	// 如果实体存在，执行更新；否则执行插入
	if exists {
		return r.Update(ctx, entity)
	} else {
		return r.insert(ctx, entity)
	}
}

// insert 插入新实体
func (r *PostgresRepository[T]) insert(ctx context.Context, entity T) error {
	columns, values, _ := r.buildInsertData(entity)

	// 构建参数占位符
	placeholders := make([]string, len(columns))
	for i := range columns {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (%s) 
		VALUES (%s) 
		RETURNING id`,
		r.tableName, strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	// 使用QueryContext和我们处理过的值
	var insertedID string
	err := r.db.QueryRowContext(ctx, query, values...).Scan(&insertedID)
	if err != nil {
		return fmt.Errorf("failed to insert entity: %w", err)
	}

	// 确保返回的ID与实体的ID匹配
	if insertedID != entity.GetID() {
		return fmt.Errorf("inserted ID (%s) does not match entity ID (%s)", insertedID, entity.GetID())
	}

	return nil
}

// Update 更新实体
func (r *PostgresRepository[T]) Update(ctx context.Context, entity T) error {
	if entity.GetID() == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "update",
			Message:    "entity ID cannot be empty",
		}
	}

	entity.SetUpdatedAt(time.Now())

	columns, values := r.buildUpdateData(entity)

	query := fmt.Sprintf(`
		UPDATE %s 
		SET %s 
		WHERE id = $1`,
		r.tableName, strings.Join(columns, ", "))

	values = append([]interface{}{entity.GetID()}, values...)

	_, err := r.db.ExecContext(ctx, query, values...)
	return err
}

// FindByID 根据ID查找实体
func (r *PostgresRepository[T]) FindByID(ctx context.Context, id string) (T, error) {
	var zero T
	if id == "" {
		return zero, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_id",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}

	query := fmt.Sprintf(`SELECT * FROM %s WHERE id = $1 AND deleted_at IS NULL`, r.tableName)

	// 创建一个新的实体实例
	var entity T

	// 使用反射来获取实体的实际类型并创建实例
	var entityPtr interface{}
	var zeroValue T
	entityType := reflect.TypeOf(zeroValue)

	if entityType.Kind() == reflect.Ptr {
		// 如果是指针类型，创建一个新的实体指针
		entityValue := reflect.New(entityType.Elem())
		entityPtr = entityValue.Interface()
	} else {
		// 如果是值类型，创建一个指向该值的指针
		entityPtr = reflect.New(entityType).Interface()
	}

	// 执行查询，使用QueryxContext来获取*sqlx.Rows
	rows, err := r.db.QueryxContext(ctx, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return zero, &errs.RepositoryError{
				EntityType: r.entityType,
				Operation:  "find_by_id",
				ID:         id,
				Message:    "entity not found",
			}
		}
		return zero, err
	}
	defer rows.Close()

	// 检查是否有结果
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return zero, err
		}
		return zero, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_id",
			ID:         id,
			Message:    "entity not found",
		}
	}

	// 使用自定义的扫描方法
	err = r.scanEntity(ctx, rows, entityPtr)
	if err != nil {
		return zero, err
	}

	// 确保rows已经扫描完毕
	if rows.Next() {
		return zero, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_id",
			ID:         id,
			Message:    "multiple entities found with the same ID",
		}
	}

	// 将查询结果转换回T类型
	if entityType.Kind() == reflect.Ptr {
		entity = entityPtr.(T)
	} else {
		entity = reflect.ValueOf(entityPtr).Elem().Interface().(T)
	}

	return entity, nil
}

// FindAll 查找所有实体
func (r *PostgresRepository[T]) FindAll(ctx context.Context) ([]T, error) {
	query := fmt.Sprintf(`SELECT * FROM %s WHERE deleted_at IS NULL ORDER BY created_at DESC`, r.tableName)

	var entities []T
	var zeroValue T
	entityType := reflect.TypeOf(zeroValue)

	// 执行查询
	rows, err := r.db.QueryxContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 处理每一行数据
	for rows.Next() {
		// 创建一个新的实体实例
		var entityPtr interface{}
		if entityType.Kind() == reflect.Ptr {
			// 如果是指针类型，创建一个新的实体指针
			entityValue := reflect.New(entityType.Elem())
			entityPtr = entityValue.Interface()
		} else {
			// 如果是值类型，创建一个指向该值的指针
			entityPtr = reflect.New(entityType).Interface()
		}

		// 使用自定义的扫描方法
		err := r.scanEntity(ctx, rows, entityPtr)
		if err != nil {
			return nil, err
		}

		// 将扫描结果添加到结果集
		var entity T
		if entityType.Kind() == reflect.Ptr {
			entity = entityPtr.(T)
		} else {
			entity = reflect.ValueOf(entityPtr).Elem().Interface().(T)
		}
		entities = append(entities, entity)
	}

	// 检查扫描过程中是否有错误
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entities, nil
}

// Delete 删除实体（软删除）
func (r *PostgresRepository[T]) Delete(ctx context.Context, id string) error {
	if id == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "delete",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}

	query := fmt.Sprintf(`
		UPDATE %s 
		SET deleted_at = NOW() 
		WHERE id = $1 AND deleted_at IS NULL`, r.tableName)

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "delete",
			ID:         id,
			Message:    "entity not found",
		}
	}

	return nil
}

// FindByIDs 根据多个ID查找实体
func (r *PostgresRepository[T]) FindByIDs(ctx context.Context, ids []string) ([]T, error) {
	if len(ids) == 0 {
		return []T{}, nil
	}

	// 构建带有占位符的查询，避免SQL注入
	placeholders := make([]string, len(ids))
	params := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		params[i] = id
	}

	query := fmt.Sprintf(
		`
			SELECT * FROM %s 
			WHERE id IN (%s) AND deleted_at IS NULL 
			ORDER BY created_at DESC`,
		r.tableName,
		strings.Join(placeholders, ", "))

	// 使用QueryxContext而不是直接Select，以便能够使用我们的scanEntity方法处理复杂类型
	rows, err := r.db.QueryxContext(ctx, query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []T
	for rows.Next() {
		// 创建一个新的实体实例
		var entity T
		// 使用我们改进的scanEntity方法来处理查询结果
		if err := r.scanEntity(ctx, rows, &entity); err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}

	// 检查遍历过程中是否有错误
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entities, nil
}

// DeleteBatch 批量删除实体（软删除）
func (r *PostgresRepository[T]) DeleteBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	// 构建带有占位符的查询，避免SQL注入
	placeholders := make([]string, len(ids))
	params := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		params[i] = id
	}

	query := fmt.Sprintf(`
		UPDATE %s 
		SET deleted_at = NOW() 
		WHERE id IN (%s) AND deleted_at IS NULL`,
		r.tableName,
		strings.Join(placeholders, ", "))

	_, err := r.db.ExecContext(ctx, query, params...)
	return err
}

// FindWithPagination 分页查询实体
func (r *PostgresRepository[T]) FindWithPagination(ctx context.Context, limit, offset int) ([]T, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
		SELECT * FROM %s 
		WHERE deleted_at IS NULL 
		ORDER BY created_at DESC 
		LIMIT $1 OFFSET $2`, r.tableName)

	// 使用QueryxContext而不是直接Select，以便能够使用我们的scanEntity方法处理复杂类型
	rows, err := r.db.QueryxContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []T
	for rows.Next() {
		// 创建一个新的实体实例
		var entity T
		// 使用我们改进的scanEntity方法来处理查询结果
		if err := r.scanEntity(ctx, rows, &entity); err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}

	// 检查遍历过程中是否有错误
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entities, nil
}

// Count 统计实体总数
func (r *PostgresRepository[T]) Count(ctx context.Context) (int64, error) {
	var count int64
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE deleted_at IS NULL`, r.tableName)

	err := r.db.GetContext(ctx, &count, query)
	return count, err
}

// FindByField 按字段查找实体
func (r *PostgresRepository[T]) FindByField(ctx context.Context, field string, value interface{}) ([]T, error) {
	if field == "" {
		return nil, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_field",
			Message:    "field name cannot be empty",
		}
	}

	// 防止SQL注入，验证字段名
	if !r.isValidFieldName(field) {
		return nil, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_field",
			Message:    fmt.Sprintf("invalid field name: %s", field),
		}
	}

	query := fmt.Sprintf(
		`SELECT * FROM %s 
		WHERE %s = $1 AND deleted_at IS NULL 
		ORDER BY created_at DESC`, r.tableName, field)

	// 使用QueryxContext而不是直接Select，以便能够使用我们的scanEntity方法处理复杂类型
	rows, err := r.db.QueryxContext(ctx, query, value)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []T
	for rows.Next() {
		// 创建一个新的实体实例
		var entity T
		// 使用我们改进的scanEntity方法来处理查询结果
		if err := r.scanEntity(ctx, rows, &entity); err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}

	// 检查遍历过程中是否有错误
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entities, nil
}

// Exists 检查实体是否存在
func (r *PostgresRepository[T]) Exists(ctx context.Context, id string) (bool, error) {
	if id == "" {
		return false, nil
	}

	var exists bool
	query := fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1 FROM %s 
			WHERE id = $1 AND deleted_at IS NULL
		)`, r.tableName)

	err := r.db.GetContext(ctx, &exists, query, id)
	return exists, err
}

// 事务相关方法

// existsInTx 在事务中检查实体是否存在
func (r *PostgresRepository[T]) existsInTx(ctx context.Context, tx *sqlx.Tx, id string) (bool, error) {
	var exists bool
	query := fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1 FROM %s 
			WHERE id = $1 AND deleted_at IS NULL
		)`, r.tableName)

	err := tx.GetContext(ctx, &exists, query, id)
	return exists, err
}

// insertInTx 在事务中插入实体
func (r *PostgresRepository[T]) insertInTx(ctx context.Context, tx *sqlx.Tx, entity T) error {
	columns, values, _ := r.buildInsertData(entity)

	// 构建参数占位符
	placeholders := make([]string, len(columns))
	for i := range columns {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (%s) 
		VALUES (%s)
		RETURNING id`,
		r.tableName, strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	// 使用标准的ExecContext和我们构建的values
	var insertedID string
	err := tx.QueryRowContext(ctx, query, values...).Scan(&insertedID)
	if err != nil {
		return err
	}

	// 验证插入的ID是否与实体ID匹配
	if insertedID != entity.GetID() {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			ID:         entity.GetID(),
			Operation:  "insert",
			Message:    "inserted entity ID does not match",
			Cause:      err,
		}
	}

	return nil
}

// updateInTx 在事务中更新实体
func (r *PostgresRepository[T]) updateInTx(ctx context.Context, tx *sqlx.Tx, entity T) error {
	entity.SetUpdatedAt(time.Now())

	columns, values := r.buildUpdateData(entity)

	query := fmt.Sprintf(`
		UPDATE %s 
		SET %s 
		WHERE id = $1`,
		r.tableName, strings.Join(columns, ", "))

	values = append([]interface{}{entity.GetID()}, values...)

	_, err := tx.ExecContext(ctx, query, values...)
	return err
}

// 辅助方法

// StringArray 是一个自定义类型，用于支持PostgreSQL数组类型的扫描
func StringArray(val []string) []string {
	return val
}

// ScanStringArray 从PostgreSQL数组格式的[]uint8中解析出[]string
func ScanStringArray(data []byte) ([]string, error) {
	if data == nil || len(data) < 2 {
		return []string{}, nil
	}

	// 去掉数组的大括号
	str := string(data[1 : len(data)-1])
	if str == "" {
		return []string{}, nil
	}

	// 解析数组元素
	var result []string
	var current string
	inQuote := false

	for i := 0; i < len(str); i++ {
		if str[i] == '\'' {
			inQuote = !inQuote
		} else if str[i] == ',' && !inQuote {
			result = append(result, strings.Replace(current, "''", "'", -1))
			current = ""
		} else {
			current += string(str[i])
		}
	}

	// 添加最后一个元素
	if current != "" {
		result = append(result, strings.Replace(current, "''", "'", -1))
	}

	return result, nil
}

// buildInsertData 构建插入数据
func (r *PostgresRepository[T]) buildInsertData(entity T) ([]string, []interface{}, []string) {
	var columns []string
	var values []interface{}
	var placeholders []string

	// 使用反射获取实体的所有字段
	entityValue := reflect.ValueOf(entity)
	if entityValue.Kind() == reflect.Ptr {
		entityValue = entityValue.Elem()
	}
	entityType := entityValue.Type()

	// 确保entityType是结构体类型
	if entityType.Kind() != reflect.Struct {
		return []string{}, []interface{}{}, []string{}
	}

	for i := 0; i < entityType.NumField(); i++ {
		field := entityType.Field(i)
		dbTag := field.Tag.Get("db")
		if dbTag != "" && dbTag != "-" {
			columns = append(columns, dbTag)
			placeholders = append(placeholders, ":"+dbTag)

			// 获取字段值并添加到values切片中
			fieldValue := entityValue.Field(i)
			// 确保字段是可导出的（首字母大写）
			if fieldValue.CanInterface() {
				value := fieldValue.Interface()
				// 处理特殊类型
				handledValue, err := handleSpecialType(value)
				if err != nil {
					// 如果处理失败，使用原始值
					values = append(values, value)
				} else {
					values = append(values, handledValue)
				}
			}
		}
	}

	// 只设置更新时间，因为Entity接口没有SetCreatedAt方法
	entity.SetUpdatedAt(time.Now())

	return columns, values, placeholders
}

// buildUpdateData 构建更新数据
func (r *PostgresRepository[T]) buildUpdateData(entity T) ([]string, []interface{}) {
	var columns []string
	var values []interface{}

	// 使用反射获取实体的所有字段
	entityValue := reflect.ValueOf(entity)
	entityType := entityValue.Type()

	// 处理多级指针类型
	derefValue, derefType, err := r.dereferencePointer(entityValue)
	if err != nil {
		return []string{}, []interface{}{}
	}
	entityValue = derefValue
	entityType = derefType

	// 确保entityType是结构体类型
	if entityType.Kind() != reflect.Struct {
		return []string{}, []interface{}{}
	}

	// 设置更新时间
	entity.SetUpdatedAt(time.Now())

	// 计数器，用于参数索引
	paramIndex := 2

	for i := 0; i < entityType.NumField(); i++ {
		field := entityType.Field(i)
		dbTag := field.Tag.Get("db")
		if dbTag != "" && dbTag != "-" && dbTag != "id" && dbTag != "created_at" && dbTag != "deleted_at" {
			columns = append(columns, fmt.Sprintf("%s = $%d", dbTag, paramIndex))

			// 获取字段值并添加到values切片中
			fieldValue := entityValue.Field(i)
			// 确保字段是可导出的（首字母大写）
			if fieldValue.CanInterface() {
				value := fieldValue.Interface()
				// 处理特殊类型
				handledValue, err := handleSpecialType(value)
				if err != nil {
					// 如果处理失败，使用原始值
					values = append(values, value)
				} else {
					values = append(values, handledValue)
				}
			}
			paramIndex++
		}
	}

	return columns, values
}

// handleSpecialType 处理特殊类型，如[]string和map[string]interface{}
func handleSpecialType(value interface{}) (interface{}, error) {
	// 处理[]string类型
	sliceValue := reflect.ValueOf(value)
	if sliceValue.Kind() == reflect.Slice && sliceValue.Type().Elem().Kind() == reflect.String {
		// 将[]string转换为PostgreSQL数组格式
		var pgArray string
		if sliceValue.Len() == 0 {
			pgArray = "{}"
		} else {
			pgArray = arrayToString(sliceValue.Interface().([]string))
		}
		return pgArray, nil
	}

	// 处理map[string]interface{}类型
	if m, ok := value.(map[string]interface{}); ok {
		// 将map转换为JSON字符串
		jsonBytes, err := json.Marshal(m)
		if err != nil {
			return nil, err
		}
		return string(jsonBytes), nil
	}

	// 其他类型返回原值
	return value, nil
}

// arrayToString 将字符串切片转换为PostgreSQL数组格式的字符串
func arrayToString(arr []string) string {
	if len(arr) == 0 {
		return "{}"
	}
	result := "{"
	for i, s := range arr {
		// 转义字符串中的单引号
		escaped := strings.Replace(s, "'", "''", -1)
		result += escaped
		if i < len(arr)-1 {
			result += ","
		}
	}
	result += "}"
	return result
}

// isValidFieldName 验证字段名是否有效
func (r *PostgresRepository[T]) isValidFieldName(field string) bool {
	// 简单的字段名验证，防止SQL注入
	validFields := map[string]bool{
		"id":         true,
		"name":       true,
		"email":      true,
		"status":     true,
		"priority":   true,
		"created_at": true,
		"updated_at": true,
		"deleted_at": true,
	}
	return validFields[field]
}

// HardDelete 硬删除（永久删除）
func (r *PostgresRepository[T]) HardDelete(ctx context.Context, id string) error {
	if id == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "hard_delete",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}

	query := fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, r.tableName)

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "hard_delete",
			ID:         id,
			Message:    "entity not found",
		}
	}

	return nil
}

// FindDeleted 查找已删除的实体（包括软删除的）
func (r *PostgresRepository[T]) FindDeleted(ctx context.Context, limit, offset int) ([]T, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
		SELECT * FROM %s 
		WHERE deleted_at IS NOT NULL 
		ORDER BY deleted_at DESC 
		LIMIT $1 OFFSET $2`, r.tableName)

	var entities []T
	err := r.db.SelectContext(ctx, &entities, query, limit, offset)
	return entities, err
}

// Restore 恢复已删除的实体（取消软删除）
func (r *PostgresRepository[T]) Restore(ctx context.Context, id string) error {
	if id == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "restore",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}

	query := fmt.Sprintf(`
		UPDATE %s 
		SET deleted_at = NULL 
		WHERE id = $1 AND deleted_at IS NOT NULL`, r.tableName)

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "restore",
			ID:         id,
			Message:    "entity not found or not deleted",
		}
	}

	return nil
}

// FindByFieldWithPagination 按字段查找并支持分页
func (r *PostgresRepository[T]) FindByFieldWithPagination(ctx context.Context, field string, value interface{}, limit, offset int) ([]T, error) {
	if field == "" {
		return nil, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_field_with_pagination",
			Message:    "field name cannot be empty",
		}
	}
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	// 防止SQL注入，验证字段名
	if !r.isValidFieldName(field) {
		return nil, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_field_with_pagination",
			Message:    fmt.Sprintf("invalid field name: %s", field),
		}
	}

	query := fmt.Sprintf(`
		SELECT * FROM %s 
		WHERE %s = $1 AND deleted_at IS NULL 
		ORDER BY created_at DESC 
		LIMIT $2 OFFSET $3`, r.tableName, field)

	// 使用QueryxContext而不是直接Select，以便能够使用我们的scanEntity方法处理复杂类型
	rows, err := r.db.QueryxContext(ctx, query, value, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []T
	for rows.Next() {
		// 创建一个新的实体实例
		var entity T
		// 使用我们改进的scanEntity方法来处理查询结果
		if err := r.scanEntity(ctx, rows, &entity); err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}

	// 检查遍历过程中是否有错误
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entities, nil
}

// CountByField 按字段统计数量
func (r *PostgresRepository[T]) CountByField(ctx context.Context, field string, value interface{}) (int64, error) {
	if field == "" {
		return 0, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "count_by_field",
			Message:    "field name cannot be empty",
		}
	}

	// 防止SQL注入，验证字段名
	if !r.isValidFieldName(field) {
		return 0, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "count_by_field",
			Message:    fmt.Sprintf("invalid field name: %s", field),
		}
	}

	var count int64
	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM %s 
		WHERE %s = $1 AND deleted_at IS NULL`, r.tableName, field)

	err := r.db.GetContext(ctx, &count, query, value)
	return count, err
}

// FindWithConditions 根据条件查询
// FindWithConditions 根据条件查询
func (r *PostgresRepository[T]) FindWithConditions(ctx context.Context, options usecase.QueryOptions) ([]T, error) {
	if options.Limit <= 0 {
		options.Limit = 10
	}
	if options.Offset < 0 {
		options.Offset = 0
	}

	// 构建WHERE子句和参数
	whereClause, params := r.buildWhereClause(options.Conditions)

	// 构建ORDER BY子句
	orderByClause := r.buildOrderByClause(options.SortFields)

	// 构建完整查询
	query := fmt.Sprintf(`
		SELECT * FROM %s 
		%s 
		%s 
		LIMIT $%d OFFSET $%d`,
		r.tableName, whereClause, orderByClause, len(params)+1, len(params)+2)

	// 添加分页参数
	params = append(params, options.Limit, options.Offset)

	// 使用自定义的扫描逻辑来处理特殊类型
	rows, err := r.db.QueryxContext(ctx, query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []T
	for rows.Next() {
		var entity T
		err := r.scanEntity(ctx, rows, &entity)
		if err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}

	return entities, nil
}

// CountWithConditions 根据条件统计
func (r *PostgresRepository[T]) CountWithConditions(ctx context.Context, conditions []usecase.QueryCondition) (int64, error) {
	var count int64

	// 构建WHERE子句和参数
	whereClause, params := r.buildWhereClause(conditions)

	// 构建完整查询
	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM %s %s`, r.tableName, whereClause)

	err := r.db.GetContext(ctx, &count, query, params...)
	return count, err
}

// buildWhereClause 构建WHERE子句
func (r *PostgresRepository[T]) buildWhereClause(conditions []usecase.QueryCondition) (string, []interface{}) {
	if len(conditions) == 0 {
		return "WHERE deleted_at IS NULL", []interface{}{}
	}

	var clauses []string
	var params []interface{}
	clauses = append(clauses, "deleted_at IS NULL")

	for i, condition := range conditions {
		// 验证字段名
		if !r.isValidFieldName(condition.Field) {
			continue
		}

		paramIndex := i + 1
		opClause, param := r.buildConditionClause(condition, paramIndex)
		if opClause != "" {
			clauses = append(clauses, opClause)
			params = append(params, param)
		}
	}

	if len(clauses) == 0 {
		return "", []interface{}{}
	}

	return "WHERE " + strings.Join(clauses, " AND "), params
}

// buildConditionClause 构建单个条件子句
func (r *PostgresRepository[T]) buildConditionClause(condition usecase.QueryCondition, paramIndex int) (string, interface{}) {
	field := condition.Field
	op := condition.Operator
	value := condition.Value

	switch op {
	case "EQ":
		return fmt.Sprintf("%s = $%d", field, paramIndex), value
	case "NEQ":
		return fmt.Sprintf("%s <> $%d", field, paramIndex), value
	case "GT":
		return fmt.Sprintf("%s > $%d", field, paramIndex), value
	case "GTE":
		return fmt.Sprintf("%s >= $%d", field, paramIndex), value
	case "LT":
		return fmt.Sprintf("%s < $%d", field, paramIndex), value
	case "LTE":
		return fmt.Sprintf("%s <= $%d", field, paramIndex), value
	case "LIKE":
		return fmt.Sprintf("%s ILIKE $%d", field, paramIndex), fmt.Sprintf("%%%v%%", value)
	case "IN":
		// 处理IN操作符，使用pq.Array来支持数组参数
		if slice, ok := value.([]interface{}); ok {
			// 将[]interface{}转换为具体类型的切片（例如[]string）
			// 这里简化处理，假设值都是字符串类型
			var strSlice []string
			for _, v := range slice {
				strSlice = append(strSlice, fmt.Sprintf("%v", v))
			}
			return fmt.Sprintf("%s = ANY($%d)", field, paramIndex), strSlice
		}
		return "", nil
	case "IS_NULL":
		return fmt.Sprintf("%s IS NULL", field), nil
	case "IS_NOT_NULL":
		return fmt.Sprintf("%s IS NOT NULL", field), nil
	default:
		return "", nil
	}
}

// buildOrderByClause 构建ORDER BY子句
func (r *PostgresRepository[T]) buildOrderByClause(sortFields []usecase.SortField) string {
	if len(sortFields) == 0 {
		return "ORDER BY created_at DESC"
	}

	var clauses []string
	for _, sortField := range sortFields {
		if !r.isValidFieldName(sortField.Field) {
			continue
		}

		direction := "ASC"
		if !sortField.Ascending {
			direction = "DESC"
		}

		clauses = append(clauses, fmt.Sprintf("%s %s", sortField.Field, direction))
	}

	if len(clauses) == 0 {
		return "ORDER BY created_at DESC"
	}

	return "ORDER BY " + strings.Join(clauses, ", ")
}

// scanEntity 使用自定义逻辑扫描实体
// 处理一些特殊类型，如[]string等
func (r *PostgresRepository[T]) scanEntity(ctx context.Context, rows *sqlx.Rows, entity interface{}) error {
	// 获取实体的值和类型
	entityValue := reflect.ValueOf(entity)
	entityType := entityValue.Type()

	// 特殊处理双重指针的情况（如 &entity 其中 entity 是 *PostgresComplexEntity 类型）
	// 在FindWithConditions方法中就是这种情况
	if entityType.Kind() == reflect.Ptr {
		ptrElemType := entityType.Elem()
		// 如果是指向指针的指针
		if ptrElemType.Kind() == reflect.Ptr {
			// 获取最内层指针指向的类型
			innerElemType := ptrElemType.Elem()
			// 如果内部指针是nil，我们需要先创建一个新的实例
			if entityValue.Elem().IsZero() {
				// 创建一个新的最内层类型的实例
				newValue := reflect.New(innerElemType)
				// 将新创建的实例赋值给内部指针
				entityValue.Elem().Set(newValue)
			}
			// 现在我们可以安全地解引用到内部指针
			entityValue = entityValue.Elem()
			entityType = ptrElemType
		}
	}

	// 处理所有级别的指针类型，直到得到结构体
	for entityType.Kind() == reflect.Ptr {
		// 确保指针不是nil
		if entityValue.IsZero() {
			return fmt.Errorf("cannot dereference nil pointer")
		}
		// 解引用指针
		entityValue = entityValue.Elem()
		entityType = entityValue.Type()
	}

	// 确保最终得到的是结构体类型
	if entityType.Kind() != reflect.Struct {
		return fmt.Errorf("entity must be a struct type, got %v", entityType.Kind())
	}

	// 获取列信息
	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	// 创建值指针数组
	values := make([]interface{}, len(columns))
	for i := range columns {
		// 创建一个指针，用于存储扫描结果
		values[i] = reflect.New(reflect.TypeOf([]byte{})).Interface()
	}

	// 扫描行数据
	if err := rows.Scan(values...); err != nil {
		return err
	}

	// 创建列名到字段名的映射
	columnToField := make(map[string]string)
	for i := 0; i < entityType.NumField(); i++ {
		field := entityType.Field(i)
		dbTag := field.Tag.Get("db")
		if dbTag != "" {
			columnToField[dbTag] = field.Name
		}
	}

	// 处理扫描结果
	for i, col := range columns {
		// 查找对应的字段
		fieldName, ok := columnToField[col]
		if !ok {
			continue
		}

		fieldValue := entityValue.FieldByName(fieldName)
		if !fieldValue.IsValid() || !fieldValue.CanSet() {
			continue
		}

		// 获取扫描的数据
		data := values[i].(*[]byte)
		if data == nil || *data == nil {
			// 如果数据为nil，设置字段为零值
			fieldValue.Set(reflect.Zero(fieldValue.Type()))
			continue
		}

		// 根据字段类型进行不同的处理
		switch fieldValue.Kind() {
		case reflect.String:
			// 处理string类型
			fieldValue.SetString(string(*data))
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			// 处理整数类型
			var num int64
			if n, err := strconv.ParseInt(string(*data), 10, 64); err == nil {
				num = n
			}
			fieldValue.SetInt(num)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			// 处理无符号整数类型
			var num uint64
			if n, err := strconv.ParseUint(string(*data), 10, 64); err == nil {
				num = n
			}
			fieldValue.SetUint(num)
		case reflect.Float32, reflect.Float64:
			// 处理浮点数类型
			var num float64
			if n, err := strconv.ParseFloat(string(*data), 64); err == nil {
				num = n
			}
			fieldValue.SetFloat(num)
		case reflect.Bool:
			// 处理bool类型
			var b bool
			if v, err := strconv.ParseBool(string(*data)); err == nil {
				b = v
			}
			fieldValue.SetBool(b)
		case reflect.Slice:
			// 处理[]string类型
			if fieldValue.Type().Elem().Kind() == reflect.String {
				// 将PostgreSQL数组转换为[]string
				strArray, err := ScanStringArray(*data)
				if err == nil {
					fieldValue.Set(reflect.ValueOf(strArray))
				} else {
					// 如果解析失败，使用空数组
					fieldValue.Set(reflect.ValueOf([]string{}))
				}
			}
		case reflect.Map:
			// 处理map类型
			if fieldValue.Type().Key().Kind() == reflect.String && fieldValue.Type().Elem().Kind() == reflect.Interface {
				// 解析JSON字符串到map
				var m map[string]interface{}
				if err := json.Unmarshal(*data, &m); err == nil {
					fieldValue.Set(reflect.ValueOf(m))
				}
			}
		case reflect.Struct:
			// 处理结构体类型，主要是time.Time
			if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
				// 解析时间字符串
				if t, err := time.Parse(time.RFC3339, string(*data)); err == nil {
					fieldValue.Set(reflect.ValueOf(t))
				} else if t, err := time.Parse("2006-01-02 15:04:05", string(*data)); err == nil {
					fieldValue.Set(reflect.ValueOf(t))
				} else if t, err := time.Parse("2006-01-02", string(*data)); err == nil {
					fieldValue.Set(reflect.ValueOf(t))
				}
			}
		case reflect.Ptr:
			// 处理指针类型，主要是*time.Time
			if fieldValue.Type().Elem() == reflect.TypeOf(time.Time{}) {
				// 解析时间字符串
				if t, err := time.Parse(time.RFC3339, string(*data)); err == nil {
					ptr := reflect.New(reflect.TypeOf(time.Time{}))
					ptr.Elem().Set(reflect.ValueOf(t))
					fieldValue.Set(ptr)
				} else if t, err := time.Parse("2006-01-02 15:04:05", string(*data)); err == nil {
					ptr := reflect.New(reflect.TypeOf(time.Time{}))
					ptr.Elem().Set(reflect.ValueOf(t))
					fieldValue.Set(ptr)
				} else if t, err := time.Parse("2006-01-02", string(*data)); err == nil {
					ptr := reflect.New(reflect.TypeOf(time.Time{}))
					ptr.Elem().Set(reflect.ValueOf(t))
					fieldValue.Set(ptr)
				} else {
					// 如果解析失败，设置为nil
					fieldValue.Set(reflect.Zero(fieldValue.Type()))
				}
			}
		}
	}

	return nil
}
