package entity

import "testing"

// TestGeneratedIDsAreUnique 验证连续创建的事务与任务ID不会重复
func TestGeneratedIDsAreUnique(t *testing.T) {
	tx1 := NewTransaction("tx1", nil)
	tx2 := NewTransaction("tx2", nil)
	if tx1.ID == "" {
		t.Error("事务ID不能为空")
	}
	if tx1.ID == tx2.ID {
		t.Errorf("连续创建的事务ID不能相同: %s", tx1.ID)
	}

	task1 := NewTask("task1", nil)
	task2 := NewTask("task2", nil)
	if task1.Id == "" {
		t.Error("任务ID不能为空")
	}
	if task1.Id == task2.Id {
		t.Errorf("连续创建的任务ID不能相同: %s", task1.Id)
	}
}
