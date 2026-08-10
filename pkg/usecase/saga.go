package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/DotNetAge/sparrow/pkg/entity"
)

// SagaService Saga事务服务
// 位于整洁架构的用例层 - 包含核心业务逻辑
// 不依赖具体实现，只依赖抽象接口

type SagaService struct {
	repo         Repository[*entity.Transaction]
	handlers     map[string]HandlerFunc
	compensators map[string]HandlerFunc
}

type HandlerFunc func(ctx context.Context, payload map[string]interface{}) error

// NewSagaService 创建Saga服务实例
func NewSagaService(repo Repository[*entity.Transaction]) *SagaService {
	return &SagaService{
		repo:         repo,
		handlers:     make(map[string]HandlerFunc),
		compensators: make(map[string]HandlerFunc),
	}
}

// RegisterHandler 注册事务步骤处理器
func (s *SagaService) RegisterHandler(name string, handler HandlerFunc) {
	s.handlers[name] = handler
}

// RegisterCompensator 注册补偿处理器
func (s *SagaService) RegisterCompensator(name string, compensator HandlerFunc) {
	s.compensators[name] = compensator
}

// ExecuteTransaction 执行事务
// 核心业务逻辑：按顺序执行事务步骤，失败时触发补偿
func (s *SagaService) ExecuteTransaction(ctx context.Context, name string, steps []entity.Step) (*entity.Transaction, error) {
	// 创建事务实体
	tx := entity.NewTransaction(name, steps)

	// 保存初始状态
	if err := s.repo.Save(ctx, tx); err != nil {
		return nil, fmt.Errorf("failed to save transaction: %w", err)
	}

	// 开始执行
	tx.Status = entity.StatusRunning
	if err := s.repo.Save(ctx, tx); err != nil {
		return nil, fmt.Errorf("failed to update transaction status: %w", err)
	}

	// 按顺序执行步骤
	for i, step := range tx.Steps {
		if err := s.executeStep(ctx, tx, i, step); err != nil {
			// 执行失败，触发补偿
			if compErr := s.compensate(ctx, tx, i-1); compErr != nil {
				return tx, fmt.Errorf("transaction failed at step %d: %w; compensation failed: %w", i, err, compErr)
			}
			return tx, fmt.Errorf("transaction failed at step %d: %w", i, err)
		}
	}

	// 所有步骤执行成功
	tx.Status = entity.StatusCompleted
	if err := s.repo.Save(ctx, tx); err != nil {
		return nil, fmt.Errorf("failed to complete transaction: %w", err)
	}

	return tx, nil
}

// executeStep 执行单个步骤
func (s *SagaService) executeStep(ctx context.Context, tx *entity.Transaction, stepIndex int, step entity.Step) error {
	handler, ok := s.handlers[step.Handler]
	if !ok {
		return fmt.Errorf("no handler registered for: %s", step.Handler)
	}

	// 执行步骤
	if err := handler(ctx, step.Payload); err != nil {
		return err
	}

	// 更新进度
	tx.Current = stepIndex + 1
	return s.repo.Save(ctx, tx)
}

// compensate 执行补偿，返回补偿过程中发生的所有错误
func (s *SagaService) compensate(ctx context.Context, tx *entity.Transaction, failedStepIndex int) error {
	tx.Status = entity.StatusFailed

	var compErrs []error
	// 反向执行补偿
	for i := failedStepIndex; i >= 0; i-- {
		step := tx.Steps[i]
		if step.Compensate == "" {
			continue
		}

		if compensator, ok := s.compensators[step.Compensate]; ok {
			if err := compensator(ctx, step.Payload); err != nil {
				compErrs = append(compErrs, fmt.Errorf("step %d compensate %s: %w", i, step.Compensate, err))
			}
		}
	}

	tx.Status = entity.StatusCompensated
	if err := s.repo.Save(ctx, tx); err != nil {
		compErrs = append(compErrs, fmt.Errorf("save compensated transaction: %w", err))
	}

	return errors.Join(compErrs...)
}

// GetTransaction 获取事务状态
func (s *SagaService) GetTransaction(ctx context.Context, id string) (*entity.Transaction, error) {
	tx, err := s.repo.FindByID(ctx, id)
	return tx, err
}
