package repo

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/errs"
	"github.com/DotNetAge/sparrow/pkg/usecase"
)

// SqlDBRepository 基于database/sql的泛型仓储实现
type SqlDBRepository[T entity.Entity] struct {
	usecase.BaseRepository[T]
	db         *sql.DB
	tableName  string
	entityType string
	model      T
}

// hasSoftDelete 检查实体是否支持软删除
func (r *SqlDBRepository[T]) hasSoftDelete() bool {
	entityValue := reflect.ValueOf(r.model)
	if entityValue.Kind() == reflect.Ptr {
		entityValue = entityValue.Elem()
	}
	return entityValue.FieldByName("DeletedAt").IsValid()
}

var _ usecase.Repository[entity.Entity] = (*SqlDBRepository[entity.Entity])(nil)

// NewSqlDBRepository 创建SqlDB仓储实例
func NewSqlDBRepository[T entity.Entity](db *sql.DB) *SqlDBRepository[T] {
	var model T
	entityType := fmt.Sprintf("%T", model)

	// 生成表名：只使用类型名的最后一部分，转换为小写
	// 例如：repo.TestEntity -> testentity, *repo.TestEntity -> testentity
	parts := strings.Split(strings.ReplaceAll(entityType, "*", ""), ".")
	typeName := parts[len(parts)-1]
	tableName := strings.ToLower(typeName)

	// 正确初始化model，特别是当T是指针类型时
	modelValue := reflect.ValueOf(model)
	if modelValue.Kind() == reflect.Ptr && modelValue.IsNil() {
		// 如果是nil指针，创建一个新实例
		modelValue = reflect.New(modelValue.Type().Elem())
		model = modelValue.Interface().(T)
	}

	return &SqlDBRepository[T]{
		BaseRepository: *usecase.NewBaseRepository[T](),
		db:             db,
		tableName:      tableName,
		entityType:     entityType,
		model:          model,
	}
}

// Save 保存实体（插入或更新）
func (r *SqlDBRepository[T]) Save(ctx context.Context, entity T) error {
	if entity.GetID() == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "save",
			Message:    "entity ID cannot be empty",
		}
	}

	// 检查是否存在
	exists, err := r.Exists(ctx, entity.GetID())
	if err != nil {
		return fmt.Errorf("failed to check existence: %w", err)
	}

	// 开始事务
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
	}()

	if exists {
		// 更新实体
		entity.SetUpdatedAt(time.Now())
		err = r.updateEntity(ctx, tx, entity)
	} else {
		// 插入新实体
		// 设置创建时间和更新时间
		entityValue := reflect.ValueOf(entity)
		if entityValue.Kind() == reflect.Ptr {
			entityValue = entityValue.Elem()
		}

		createdAtField := entityValue.FieldByName("CreatedAt")
		if createdAtField.IsValid() && createdAtField.CanSet() && createdAtField.Type() == reflect.TypeOf(time.Time{}) {
			createdAtField.Set(reflect.ValueOf(time.Now()))
		}

		entity.SetUpdatedAt(time.Now())
		err = r.insertEntity(ctx, tx, entity)
	}

	return err
}

// FindByID 根据ID查找实体
func (r *SqlDBRepository[T]) FindByID(ctx context.Context, id string) (T, error) {
	var entity T

	if id == "" {
		return entity, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_id",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}

	// 构建查询语句
	query := fmt.Sprintf("SELECT * FROM %s WHERE id = ?", r.tableName)
	args := []interface{}{id}

	// 如果支持软删除，添加条件
	if r.hasSoftDelete() {
		query += " AND deleted_at IS NULL"
	}

	// 执行查询
	err := r.scanEntity(ctx, r.db, &entity, query, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity, &errs.RepositoryError{
				EntityType: r.entityType,
				Operation:  "find_by_id",
				ID:         id,
				Message:    "entity not found",
			}
		}
		return entity, err
	}

	return entity, nil
}

// FindAll 查找所有实体
func (r *SqlDBRepository[T]) FindAll(ctx context.Context) ([]T, error) {
	var entities []T

	// 构建查询语句
	query := fmt.Sprintf("SELECT * FROM %s", r.tableName)

	// 如果支持软删除，添加条件
	if r.hasSoftDelete() {
		query += " WHERE deleted_at IS NULL"
	}

	// 执行查询
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 扫描结果
	for rows.Next() {
		var entity T
		err := r.scanRow(rows, &entity)
		if err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entities, nil
}

// Update 更新实体
func (r *SqlDBRepository[T]) Update(ctx context.Context, entity T) error {
	if entity.GetID() == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "update",
			Message:    "entity ID cannot be empty",
		}
	}

	// 检查实体是否存在
	exists, err := r.Exists(ctx, entity.GetID())
	if err != nil {
		return err
	}

	if !exists {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "update",
			ID:         entity.GetID(),
			Message:    "entity not found",
		}
	}

	// 更新时间
	entity.SetUpdatedAt(time.Now())

	// 执行更新
	return r.updateEntity(ctx, r.db, entity)
}

// Delete 删除实体（软删除，如果实体支持）
func (r *SqlDBRepository[T]) Delete(ctx context.Context, id string) error {
	if id == "" {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "delete",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}

	// 检查实体是否存在
	exists, err := r.Exists(ctx, id)
	if err != nil {
		return err
	}

	if !exists {
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "delete",
			ID:         id,
			Message:    "entity not found",
		}
	}

	// 检查实体是否有DeletedAt字段
	entityValue := reflect.ValueOf(r.model)
	if entityValue.Kind() == reflect.Ptr {
		entityValue = entityValue.Elem()
	}

	deletedAtField := entityValue.FieldByName("DeletedAt")
	if deletedAtField.IsValid() && deletedAtField.CanSet() {
		// 有DeletedAt字段，执行软删除
		query := fmt.Sprintf("UPDATE %s SET deleted_at = ? WHERE id = ?", r.tableName)
		_, err := r.db.ExecContext(ctx, query, time.Now(), id)
		return err
	}

	// 没有DeletedAt字段，执行硬删除
	query := fmt.Sprintf("DELETE FROM %s WHERE id = ?", r.tableName)
	_, err = r.db.ExecContext(ctx, query, id)
	return err
}

// SaveBatch 批量保存实体
func (r *SqlDBRepository[T]) SaveBatch(ctx context.Context, entities []T) error {
	if len(entities) == 0 {
		return nil
	}

	// 检查是否所有实体都有ID
	for i, entity := range entities {
		if entity.GetID() == "" {
			return &errs.RepositoryError{
				EntityType: r.entityType,
				Operation:  "save_batch",
				Message:    fmt.Sprintf("entity ID cannot be empty at index %d", i),
			}
		}
	}

	// 开始事务
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
	}()

	// 批量保存
	for _, entity := range entities {
		// 检查是否存在
		exists, err := r.existsInTransaction(ctx, tx, entity.GetID())
		if err != nil {
			return err
		}

		if exists {
			// 更新实体
			entity.SetUpdatedAt(time.Now())
			err = r.updateEntity(ctx, tx, entity)
			if err != nil {
				return err
			}
		} else {
			// 插入新实体
			// 设置创建时间和更新时间
			entityValue := reflect.ValueOf(entity)
			if entityValue.Kind() == reflect.Ptr {
				entityValue = entityValue.Elem()
			}

			createdAtField := entityValue.FieldByName("CreatedAt")
			if createdAtField.IsValid() && createdAtField.CanSet() && createdAtField.Type() == reflect.TypeOf(time.Time{}) {
				createdAtField.Set(reflect.ValueOf(time.Now()))
			}

			entity.SetUpdatedAt(time.Now())
			err = r.insertEntity(ctx, tx, entity)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// FindByIDs 根据多个ID查找实体
func (r *SqlDBRepository[T]) FindByIDs(ctx context.Context, ids []string) ([]T, error) {
	if len(ids) == 0 {
		return []T{}, nil
	}

	// 构建IN子句的占位符
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	// 构建查询语句
	query := fmt.Sprintf("SELECT * FROM %s WHERE id IN (%s)", r.tableName, strings.Join(placeholders, ","))

	// 如果支持软删除，添加条件
	if r.hasSoftDelete() {
		query += " AND deleted_at IS NULL"
	}

	// 执行查询
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 扫描结果
	var entities []T
	for rows.Next() {
		var entity T
		err := r.scanRow(rows, &entity)
		if err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entities, nil
}

// DeleteBatch 批量删除实体
func (r *SqlDBRepository[T]) DeleteBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	// 构建IN子句的占位符
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	// 检查实体是否有DeletedAt字段
	entityValue := reflect.ValueOf(r.model)
	if entityValue.Kind() == reflect.Ptr {
		entityValue = entityValue.Elem()
	}

	deletedAtField := entityValue.FieldByName("DeletedAt")

	var err error
	if deletedAtField.IsValid() && deletedAtField.CanSet() {
		// 有DeletedAt字段，执行软删除
		query := fmt.Sprintf("UPDATE %s SET deleted_at = ? WHERE id IN (%s)", r.tableName, strings.Join(placeholders, ","))
		// 构建参数：在args前面添加time.Now()
		updateArgs := append([]interface{}{time.Now()}, args...)
		_, err = r.db.ExecContext(ctx, query, updateArgs...)
	} else {
		// 没有DeletedAt字段，执行硬删除
		query := fmt.Sprintf("DELETE FROM %s WHERE id IN (%s)", r.tableName, strings.Join(placeholders, ","))
		_, err = r.db.ExecContext(ctx, query, args...)
	}

	return err
}

// FindWithPagination 分页查询实体
func (r *SqlDBRepository[T]) FindWithPagination(ctx context.Context, limit, offset int) ([]T, error) {
	if limit <= 0 {
		limit = 10
	}
	if offset < 0 {
		offset = 0
	}

	// 构建查询语句
	query := fmt.Sprintf("SELECT * FROM %s", r.tableName)
	// 如果支持软删除，添加条件
	if r.hasSoftDelete() {
		query += " WHERE deleted_at IS NULL"
	}
	query += " LIMIT ? OFFSET ?"

	// 执行查询
	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 扫描结果
	var entities []T
	for rows.Next() {
		var entity T
		err := r.scanRow(rows, &entity)
		if err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entities, nil
}

// Count 统计实体总数
func (r *SqlDBRepository[T]) Count(ctx context.Context) (int64, error) {
	var count int64

	// 构建查询语句
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", r.tableName)

	// 如果支持软删除，添加条件
	if r.hasSoftDelete() {
		query += " WHERE deleted_at IS NULL"
	}

	// 执行查询
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// FindByField 根据指定字段的值查询
func (r *SqlDBRepository[T]) FindByField(ctx context.Context, field string, value interface{}) ([]T, error) {
	if field == "" {
		return nil, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_field",
			Message:    "field name cannot be empty",
		}
	}

	// 构建查询语句
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = ?", r.tableName, field)

	// 如果支持软删除，添加条件
	if r.hasSoftDelete() {
		query += " AND deleted_at IS NULL"
	}

	// 执行查询
	rows, err := r.db.QueryContext(ctx, query, value)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 扫描结果
	var entities []T
	for rows.Next() {
		var entity T
		err := r.scanRow(rows, &entity)
		if err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entities, nil
}

// FindByFieldWithPagination 按字段查找并支持分页
func (r *SqlDBRepository[T]) FindByFieldWithPagination(ctx context.Context, field string, value interface{}, limit, offset int) ([]T, error) {
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

	// 构建查询语句
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = ? LIMIT ? OFFSET ?", r.tableName, field)

	// 执行查询
	rows, err := r.db.QueryContext(ctx, query, value, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 扫描结果
	var entities []T
	for rows.Next() {
		var entity T
		err := r.scanRow(rows, &entity)
		if err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entities, nil
}

// CountByField 按字段统计数量
func (r *SqlDBRepository[T]) CountByField(ctx context.Context, field string, value interface{}) (int64, error) {
	if field == "" {
		return 0, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "count_by_field",
			Message:    "field name cannot be empty",
		}
	}

	var count int64

	// 构建查询语句
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = ?", r.tableName, field)

	// 执行查询
	err := r.db.QueryRowContext(ctx, query, value).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Exists 检查实体是否存在
func (r *SqlDBRepository[T]) Exists(ctx context.Context, id string) (bool, error) {
	if id == "" {
		return false, nil
	}

	var count int64

	// 构建查询语句
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE id = ?", r.tableName)
	args := []interface{}{id}

	// 如果支持软删除，添加条件
	if r.hasSoftDelete() {
		query += " AND deleted_at IS NULL"
	}

	// 执行查询
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// FindWithConditions 根据条件查询
func (r *SqlDBRepository[T]) FindWithConditions(ctx context.Context, options usecase.QueryOptions) ([]T, error) {
	if options.Limit <= 0 {
		options.Limit = 10
	}
	if options.Offset < 0 {
		options.Offset = 0
	}

	// 构建查询语句
	query := fmt.Sprintf("SELECT * FROM %s", r.tableName)
	args := []interface{}{}

	// 应用条件
	if len(options.Conditions) > 0 {
		query += " WHERE"
		for i, condition := range options.Conditions {
			if i > 0 {
				query += " AND"
			}
			sqlCondition, conditionArgs := r.buildCondition(condition)
			query += " " + sqlCondition
			args = append(args, conditionArgs...)
		}
	}

	// 应用排序
	if len(options.SortFields) > 0 {
		query += " ORDER BY"
		for i, sortField := range options.SortFields {
			if i > 0 {
				query += ","
			}
			order := "ASC"
			if !sortField.Ascending {
				order = "DESC"
			}
			query += fmt.Sprintf(" %s %s", r.toSnakeCase(sortField.Field), order)
		}
	} else {
		// 默认排序
		query += " ORDER BY created_at DESC"
	}

	// 应用分页
	query += " LIMIT ? OFFSET ?"
	args = append(args, options.Limit, options.Offset)

	// 执行查询
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 扫描结果
	var entities []T
	for rows.Next() {
		var entity T
		err := r.scanRow(rows, &entity)
		if err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entities, nil
}

// CountWithConditions 根据条件统计
func (r *SqlDBRepository[T]) CountWithConditions(ctx context.Context, conditions []usecase.QueryCondition) (int64, error) {
	var count int64

	// 构建查询语句
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", r.tableName)
	args := []interface{}{}

	// 应用条件
	if len(conditions) > 0 {
		query += " WHERE"
		for i, condition := range conditions {
			if i > 0 {
				query += " AND"
			}
			sqlCondition, conditionArgs := r.buildCondition(condition)
			query += " " + sqlCondition
			args = append(args, conditionArgs...)
		}
	}

	// 执行查询
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Random 返回随机实体
func (r *SqlDBRepository[T]) Random(ctx context.Context, take int) ([]T, error) {
	if take <= 0 {
		take = 1
	}

	// 构建查询语句
	query := fmt.Sprintf("SELECT * FROM %s", r.tableName)

	// 如果支持软删除，添加条件
	if r.hasSoftDelete() {
		query += " WHERE deleted_at IS NULL"
	}

	// 添加随机排序和限制
	query += " ORDER BY RANDOM() LIMIT ?"

	// 执行查询
	rows, err := r.db.QueryContext(ctx, query, take)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 扫描结果
	var entities []T
	for rows.Next() {
		var entity T
		err := r.scanRow(rows, &entity)
		if err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return entities, nil
}

// 辅助方法

// existsInTransaction 在事务中检查实体是否存在
func (r *SqlDBRepository[T]) existsInTransaction(ctx context.Context, tx *sql.Tx, id string) (bool, error) {
	var count int64

	// 构建查询语句
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE id = ?", r.tableName)

	// 执行查询
	err := tx.QueryRowContext(ctx, query, id).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// buildCondition 构建单个查询条件的SQL
func (r *SqlDBRepository[T]) buildCondition(condition usecase.QueryCondition) (string, []interface{}) {
	field := r.toSnakeCase(condition.Field)
	op := condition.Operator
	value := condition.Value

	switch op {
	case "EQ":
		return fmt.Sprintf("%s = ?", field), []interface{}{value}
	case "NEQ":
		return fmt.Sprintf("%s <> ?", field), []interface{}{value}
	case "GT":
		return fmt.Sprintf("%s > ?", field), []interface{}{value}
	case "GTE":
		return fmt.Sprintf("%s >= ?", field), []interface{}{value}
	case "LT":
		return fmt.Sprintf("%s < ?", field), []interface{}{value}
	case "LTE":
		return fmt.Sprintf("%s <= ?", field), []interface{}{value}
	case "LIKE":
		return fmt.Sprintf("%s LIKE ?", field), []interface{}{fmt.Sprintf("%%%v%%", value)}
	case "IN":
		// 处理切片值
		if slice, ok := value.([]interface{}); ok {
			placeholders := make([]string, len(slice))
			for i := range placeholders {
				placeholders[i] = "?"
			}
			return fmt.Sprintf("%s IN (%s)", field, strings.Join(placeholders, ",")), slice
		}
		return fmt.Sprintf("%s IN (?)", field), []interface{}{value}
	case "IS_NULL":
		return fmt.Sprintf("%s IS NULL", field), []interface{}{}
	case "IS_NOT_NULL":
		return fmt.Sprintf("%s IS NOT NULL", field), []interface{}{}
	default:
		// 默认使用等于
		return fmt.Sprintf("%s = ?", field), []interface{}{value}
	}
}

// scanEntity 从数据库扫描单个实体
func (r *SqlDBRepository[T]) scanEntity(ctx context.Context, db interface{}, entity *T, query string, args ...interface{}) error {
	var err error
	var rows *sql.Rows

	// 根据db类型选择执行方式
	switch v := db.(type) {
	case *sql.DB:
		rows, err = v.QueryContext(ctx, query, args...)
	case *sql.Tx:
		rows, err = v.QueryContext(ctx, query, args...)
	default:
		return errors.New("invalid db type")
	}

	if err != nil {
		return err
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}

	err = r.scanRow(rows, entity)
	if err != nil {
		return err
	}

	if err := rows.Err(); err != nil {
		return err
	}

	return nil
}

// scanRow 从结果集中扫描一行到实体
func (r *SqlDBRepository[T]) scanRow(rows *sql.Rows, entity *T) error {
	// 获取列名
	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	// 获取实体的值
	entityValue := reflect.ValueOf(entity).Elem()

	// 处理指针，直到获取到实际的结构体值
	for entityValue.Kind() == reflect.Ptr {
		if entityValue.IsNil() {
			// 如果指针是 nil，创建一个新的实例
			newInstance := reflect.New(entityValue.Type().Elem())
			entityValue.Set(newInstance)
		}
		entityValue = entityValue.Elem()
	}

	// 创建用于扫描的切片
	dest := make([]interface{}, len(columns))

	// 遍历列并设置扫描目标
	for i, column := range columns {
		// 尝试找到对应的字段（支持值对象）
		field, err := r.findFieldForColumn(entityValue, column)
		if err != nil {
			// 如果字段不存在或不可设置，使用匿名变量
			dest[i] = &sql.RawBytes{}
			continue
		}

		// 检查字段类型是否为 time.Time
		if field.Type() == reflect.TypeOf(time.Time{}) {
			// 对于 time.Time 类型，我们需要一个指针来处理 NULL 值
			timePtr := new(time.Time)
			dest[i] = timePtr
			// 保存字段引用，以便在扫描后设置值
			finalField := field
			// 使用闭包来保存字段引用
			dest[i] = &nullableTime{
				timePtr: timePtr,
				setValue: func(val interface{}) {
					if val != nil {
						if t, ok := val.(time.Time); ok {
							finalField.Set(reflect.ValueOf(t))
						}
					}
				},
			}
		} else {
			// 其他类型直接设置扫描目标
			dest[i] = field.Addr().Interface()
		}
	}

	// 执行扫描
	return rows.Scan(dest...)
}

// nullableTime 用于处理 SQL NULL 到 time.Time 的转换
type nullableTime struct {
	timePtr  *time.Time
	setValue func(interface{})
}

// Scan 实现 sql.Scanner 接口
func (nt *nullableTime) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	t, ok := value.(time.Time)
	if !ok {
		return fmt.Errorf("unsupported type: %T", value)
	}
	*nt.timePtr = t
	if nt.setValue != nil {
		nt.setValue(t)
	}
	return nil
}

// Value 实现 driver.Valuer 接口
func (nt *nullableTime) Value() (driver.Value, error) {
	if nt.timePtr == nil {
		return nil, nil
	}
	return *nt.timePtr, nil
}

// findFieldForColumn 查找列对应的字段，支持值对象
func (r *SqlDBRepository[T]) findFieldForColumn(entityValue reflect.Value, column string) (reflect.Value, error) {
	// 尝试直接查找字段（处理简单字段）
	camelColumn := r.toCamelCase(column)
	field := entityValue.FieldByName(camelColumn)
	if field.IsValid() && field.CanSet() {
		return field, nil
	}

	// 特殊处理ID字段，尝试"Id"形式（与BaseEntity保持一致）
	if column == "id" {
		field = entityValue.FieldByName("Id")
		if field.IsValid() && field.CanSet() {
			return field, nil
		}
	}

	// 处理包含值对象的列名
	if strings.Contains(column, "_") {
		// 尝试不同的分割点，找到正确的值对象字段
		for i := 1; i < len(column); i++ {
			if column[i] == '_' {
				// 分割列名为值对象部分和字段部分
				voPart := column[:i]
				remainingPart := column[i+1:]

				// 转换值对象部分为驼峰命名
				voFieldName := r.toCamelCase(voPart)
				voField := entityValue.FieldByName(voFieldName)

				if voField.IsValid() {
					// 确保值对象不为 nil
					if voField.Kind() == reflect.Ptr {
						if voField.IsNil() {
							// 创建值对象实例
							newInstance := reflect.New(voField.Type().Elem())
							voField.Set(newInstance)
						}
						voField = voField.Elem()
					}

					// 递归查找剩余部分对应的字段
					return r.findFieldForColumn(voField, remainingPart)
				}
			}
		}
	}

	return reflect.Value{}, fmt.Errorf("field not found: %s", column)
}

// insertEntity 插入实体到数据库
func (r *SqlDBRepository[T]) insertEntity(ctx context.Context, db interface{}, entity T) error {
	// 获取实体的值
	entityValue := reflect.ValueOf(entity)

	// 提取所有字段（包括嵌入结构体的字段）
	fields, err := r.extractFields(entityValue)
	if err != nil {
		return err
	}

	// 收集字段名和值
	var columns []string
	var values []interface{}
	var placeholders []string

	for fieldName, fieldValue := range fields {
		// 将字段名转换为下划线命名的列名
		columnName := r.toSnakeCase(fieldName)

		// 添加到列表
		columns = append(columns, columnName)
		values = append(values, fieldValue)
		placeholders = append(placeholders, "?")
	}

	// 构建插入语句
	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		r.tableName,
		strings.Join(columns, ","),
		strings.Join(placeholders, ","),
	)

	// 根据db类型选择执行方式
	var errExec error
	switch v := db.(type) {
	case *sql.DB:
		_, errExec = v.ExecContext(ctx, query, values...)
	case *sql.Tx:
		_, errExec = v.ExecContext(ctx, query, values...)
	default:
		errExec = errors.New("invalid db type")
	}

	return errExec
}

// updateEntity 更新实体到数据库
func (r *SqlDBRepository[T]) updateEntity(ctx context.Context, db interface{}, entity T) error {
	// 获取实体的值
	entityValue := reflect.ValueOf(entity)

	// 提取所有字段（包括嵌入结构体的字段）
	fields, err := r.extractFields(entityValue)
	if err != nil {
		return err
	}

	// 收集字段名和值，排除id字段
	var updates []string
	var values []interface{}

	for fieldName, fieldValue := range fields {
		// 跳过id字段
		if fieldName == "ID" || fieldName == "Id" {
			continue
		}

		// 将字段名转换为下划线命名的列名
		columnName := r.toSnakeCase(fieldName)

		// 添加到列表
		updates = append(updates, fmt.Sprintf("%s = ?", columnName))
		values = append(values, fieldValue)
	}

	// 添加id参数
	values = append(values, entity.GetID())

	// 构建更新语句
	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE id = ?",
		r.tableName,
		strings.Join(updates, ","),
	)

	// 根据db类型选择执行方式
	var errExec error
	switch v := db.(type) {
	case *sql.DB:
		_, errExec = v.ExecContext(ctx, query, values...)
	case *sql.Tx:
		_, errExec = v.ExecContext(ctx, query, values...)
	default:
		errExec = errors.New("invalid db type")
	}

	return errExec
}

// toSnakeCase 将驼峰命名转换为下划线命名
func (r *SqlDBRepository[T]) toSnakeCase(s string) string {
	// 特殊处理ID
	if s == "ID" {
		return "id"
	}

	var result strings.Builder
	n := len(s)

	// 遍历字符串，处理每个字符
	for i, c := range s {
		if i > 0 && unicode.IsUpper(c) {
			// 检查前一个字符是否是下划线
			if i > 0 && s[i-1] != '_' {
				// 检查是否是连续大写字母的情况（如HTTP）
				if i+1 < n && unicode.IsUpper(rune(s[i+1])) {
					// 如果下一个字符也是大写，且不是最后一个字符，则不添加下划线
					// 只在当前字符是连续大写字母的最后一个时添加下划线
					if i+2 >= n || !unicode.IsUpper(rune(s[i+2])) {
						result.WriteRune('_')
					}
				} else {
					result.WriteRune('_')
				}
			}
		}
		result.WriteRune(unicode.ToLower(c))
	}
	return result.String()
}

// toCamelCase 将下划线命名转换为帕斯卡命名（首字母大写的驼峰命名）
func (r *SqlDBRepository[T]) toCamelCase(s string) string {
	var result strings.Builder
	words := strings.Split(s, "_")
	for _, word := range words {
		if word == "id" {
			// 特殊处理id，转换为ID（与测试实体保持一致）
			result.WriteString("ID")
		} else {
			result.WriteString(strings.Title(word))
		}
	}
	return result.String()
}

// extractFields 递归提取结构体及其嵌入结构体、值对象的所有字段
func (r *SqlDBRepository[T]) extractFields(value reflect.Value) (map[string]interface{}, error) {
	return r.extractFieldsWithPrefix(value, "")
}

// extractFieldsWithPrefix 带前缀的字段提取，用于处理值对象
func (r *SqlDBRepository[T]) extractFieldsWithPrefix(value reflect.Value, prefix string) (map[string]interface{}, error) {
	// 确保我们处理的是值而不是指针
	for value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return nil, fmt.Errorf("cannot extract fields from nil pointer")
		}
		value = value.Elem()
	}

	if value.Kind() != reflect.Struct {
		return nil, fmt.Errorf("extractFields expects a struct, got %v", value.Kind())
	}

	result := make(map[string]interface{})
	valueType := value.Type()

	for i := 0; i < value.NumField(); i++ {
		field := valueType.Field(i)
		fieldValue := value.Field(i)

		// 跳过不可导出的字段
		if !field.IsExported() {
			continue
		}

		// 检查字段是否为值对象（结构体但不是嵌入字段，且不是time.Time）
		isValueObject := !field.Anonymous && fieldValue.Kind() == reflect.Struct && fieldValue.Type() != reflect.TypeOf(time.Time{})
		if fieldValue.Kind() == reflect.Ptr {
			if !fieldValue.IsNil() {
				elem := fieldValue.Elem()
				isValueObject = !field.Anonymous && elem.Kind() == reflect.Struct && elem.Type() != reflect.TypeOf(time.Time{})
			} else {
				// 跳过nil值对象
				continue
			}
		}

		if field.Anonymous {
			// 如果是嵌入字段，递归提取其字段
			embeddedFields, err := r.extractFieldsWithPrefix(fieldValue, prefix)
			if err != nil {
				return nil, err
			}
			// 将嵌入字段的字段合并到结果中
			for name, val := range embeddedFields {
				result[name] = val
			}
		} else if isValueObject {
			// 如果是值对象，递归提取其字段并添加前缀
			voPrefix := field.Name
			if prefix != "" {
				voPrefix = prefix + "_" + field.Name
			}
			voFields, err := r.extractFieldsWithPrefix(fieldValue, voPrefix)
			if err != nil {
				return nil, err
			}
			// 将值对象的字段添加到结果中
			for name, val := range voFields {
				result[name] = val
			}
		} else {
			// 直接字段，添加到结果中
			fieldName := field.Name
			if prefix != "" {
				fieldName = prefix + "_" + field.Name
			}
			result[fieldName] = fieldValue.Interface()
		}
	}

	return result, nil
}
