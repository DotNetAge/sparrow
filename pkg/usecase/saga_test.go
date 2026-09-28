package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/DotNetAge/sparrow/pkg/entity"
)

// memoryTxRepo 基于内存的事务仓储
type memoryTxRepo struct {
	BaseRepository[*entity.Transaction]
	transactions map[string]*entity.Transaction
}

func (r *memoryTxRepo) Save(ctx context.Context, tx *entity.Transaction) error {
	r.transactions[tx.GetID()] = tx
	return nil
}

func (r *memoryTxRepo) FindByID(ctx context.Context, id string) (*entity.Transaction, error) {
	return r.transactions[id], nil
}

func (r *memoryTxRepo) Random(ctx context.Context, take int) ([]*entity.Transaction, error) {
	return nil, nil
}

// TestSagaCompensateFailurePropagated 验证补偿器失败时错误会向上传播，而不是被静默吞掉
func TestSagaCompensateFailurePropagated(t *testing.T) {
	repo := &memoryTxRepo{transactions: make(map[string]*entity.Transaction)}
	svc := NewSagaService(repo)

	// 记录补偿器是否被调用
	undoCalled := false

	svc.RegisterHandler("step1", func(ctx context.Context, payload map[string]interface{}) error {
		return nil
	})
	svc.RegisterHandler("step2", func(ctx context.Context, payload map[string]interface{}) error {
		return errors.New("step2 boom")
	})
	svc.RegisterCompensator("undo1", func(ctx context.Context, payload map[string]interface{}) error {
		undoCalled = true
		return errors.New("undo1 boom")
	})

	steps := []entity.Step{
		{Name: "s1", Handler: "step1", Compensate: "undo1"},
		{Name: "s2", Handler: "step2"},
	}

	tx, err := svc.ExecuteTransaction(context.Background(), "test-tx", steps)
	if err == nil {
		t.Fatal("期望事务执行失败")
	}

	if !undoCalled {
		t.Error("补偿器应当被调用")
	}

	// 补偿失败的信息必须出现在返回的错误中
	if !strings.Contains(err.Error(), "undo1 boom") {
		t.Errorf("期望错误包含补偿失败信息，实际: %v", err)
	}

	// 事务状态应标记为已补偿
	saved, findErr := repo.FindByID(context.Background(), tx.GetID())
	if findErr != nil {
		t.Fatalf("查询事务失败: %v", findErr)
	}
	if saved != nil && saved.Status != entity.StatusCompensated {
		t.Errorf("事务状态期望为 compensated，实际: %s", saved.Status)
	}
}
