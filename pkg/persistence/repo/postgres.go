package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/errs"
	"github.com/DotNetAge/sparrow/pkg/usecase"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"go.uber.org/zap"
)

// PostgresRepository PostgreSQL仓储实现
// 基于BaseRepository[T]的完整PostgreSQL实现
// 支持泛型实体类型，提供完整的CRUD操作

type PostgresRepository[T entity.Entity] struct {
	usecase.BaseRepository[T]
	db         *sqlx.DB
	tableName  string
	entityType string
	logger     *zap.Logger
}

// NewPostgresRepository 创建PostgreSQL仓储实例
// 参数:
//   - db: PostgreSQL数据库连接
//   - tableName: 数据库表名
//   - logger: 日志记录器
//
// 返回: 初始化的PostgreSQL仓储实例
func NewPostgresRepository[T entity.Entity](db *sqlx.DB, tableName string, logger *zap.Logger) *PostgresRepository[T] {
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
				r.logger.Error("Failed to rollback transaction", zap.Error(err))
			}
		}
	}()

	for _, entity := range entities {
		if entity.GetID() == "" {
			if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
				if r.logger != nil {
					r.logger.Error("Failed to rollback transaction", zap.Error(err))
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
					r.logger.Error("Failed to rollback transaction", zap.Error(rollbackErr))
				}
			}
			return err
		}

		if exists {
			if err := r.updateInTx(ctx, tx, entity); err != nil {
				if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
					if r.logger != nil {
						r.logger.Error("Failed to rollback transaction", zap.Error(rollbackErr))
					}
				}
				return err
			}
		} else {
			if err := r.insertInTx(ctx, tx, entity); err != nil {
				if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
					if r.logger != nil {
						r.logger.Error("Failed to rollback transaction", zap.Error(rollbackErr))
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
		return &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "save",
			Message:    "entity ID cannot be empty",
		}
	}

	// 检查实体是否存在
	exists, err := r.Exists(ctx, entity.GetID())
	if err != nil {
		return fmt.Errorf("failed to check existence: %w", err)
	}

	if exists {
		return r.Update(ctx, entity)
	}

	return r.insert(ctx, entity)
}

// insert 插入新实体
func (r *PostgresRepository[T]) insert(ctx context.Context, entity T) error {
	columns, _, placeholders := r.buildInsertData(entity)

	query := fmt.Sprintf(`
		INSERT INTO %s (%s) 
		VALUES (%s) 
		RETURNING *`,
		r.tableName, strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	_, err := r.db.NamedExecContext(ctx, query, entity)
	return err
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
		WHERE id = $1 
		RETURNING *`,
		r.tableName, strings.Join(columns, ", "))

	values = append([]interface{}{entity.GetID()}, values...)

	_, err := r.db.ExecContext(ctx, query, values...)
	return err
}

// FindByID 根据ID查找实体
func (r *PostgresRepository[T]) FindByID(ctx context.Context, id string) (T, error) {
	if id == "" {
		var zero T
		return zero, &errs.RepositoryError{
			EntityType: r.entityType,
			Operation:  "find_by_id",
			ID:         id,
			Message:    "id cannot be empty",
		}
	}

	query := fmt.Sprintf(`SELECT * FROM %s WHERE id = $1 AND deleted_at IS NULL`, r.tableName)

	var entity T
	err := r.db.GetContext(ctx, &entity, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
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
func (r *PostgresRepository[T]) FindAll(ctx context.Context) ([]T, error) {
	query := fmt.Sprintf(`SELECT * FROM %s WHERE deleted_at IS NULL ORDER BY created_at DESC`, r.tableName)

	var entities []T
	err := r.db.SelectContext(ctx, &entities, query)
	return entities, err
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

	query := fmt.Sprintf(`
		SELECT * FROM %s 
		WHERE id = ANY($1) AND deleted_at IS NULL 
		ORDER BY created_at DESC`, r.tableName)

	var entities []T
	err := r.db.SelectContext(ctx, &entities, query, pq.Array(ids))
	return entities, err
}

// DeleteBatch 批量删除实体（软删除）
func (r *PostgresRepository[T]) DeleteBatch(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	query := fmt.Sprintf(`
		UPDATE %s 
		SET deleted_at = NOW() 
		WHERE id = ANY($1) AND deleted_at IS NULL`, r.tableName)

	_, err := r.db.ExecContext(ctx, query, pq.Array(ids))
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

	var entities []T
	err := r.db.SelectContext(ctx, &entities, query, limit, offset)
	return entities, err
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

	query := fmt.Sprintf(`
		SELECT * FROM %s 
		WHERE %s = $1 AND deleted_at IS NULL 
		ORDER BY created_at DESC`, r.tableName, field)

	var entities []T
	err := r.db.SelectContext(ctx, &entities, query, value)
	return entities, err
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
	columns, _, placeholders := r.buildInsertData(entity)

	query := fmt.Sprintf(`
		INSERT INTO %s (%s) 
		VALUES (%s)`,
		r.tableName, strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	_, err := tx.NamedExecContext(ctx, query, entity)
	return err
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

// buildInsertData 构建插入数据
func (r *PostgresRepository[T]) buildInsertData(entity T) ([]string, []interface{}, []string) {
	// 这里需要根据实体的字段动态生成
	// 简化版本，实际使用时需要反射获取字段
	return []string{"id", "created_at", "updated_at"},
		[]interface{}{entity.GetID(), time.Now(), time.Now()},
		[]string{":id", ":created_at", ":updated_at"}
}

// buildUpdateData 构建更新数据
func (r *PostgresRepository[T]) buildUpdateData(entity T) ([]string, []interface{}) {
	// 这里需要根据实体的字段动态生成
	// 简化版本，实际使用时需要反射获取字段
	return []string{"updated_at = $2"}, []interface{}{time.Now()}
}

// isValidFieldName 验证字段名是否有效
func (r *PostgresRepository[T]) isValidFieldName(field string) bool {
	// 简单的字段名验证，防止SQL注入
	validFields := map[string]bool{
		"id":         true,
		"name":       true,
		"email":      true,
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

	var entities []T
	err := r.db.SelectContext(ctx, &entities, query, value, limit, offset)
	return entities, err
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

	var entities []T
	err := r.db.SelectContext(ctx, &entities, query, params...)
	return entities, err
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
			return fmt.Sprintf("%s = ANY($%d)", field, paramIndex), pq.Array(strSlice)
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
