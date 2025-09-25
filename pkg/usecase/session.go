package usecase

import (
	"context"
	"time"

	"github.com/DotNetAge/sparrow/pkg/entity"
	"github.com/DotNetAge/sparrow/pkg/errs"
	"github.com/DotNetAge/sparrow/pkg/logger"
)

// SessionService 会话服务
// 位于整洁架构的核心层 - 用例层
// 包含会话管理的所有业务逻辑

type SessionService struct {
	repo Repository[*entity.Session]
	ttl  time.Duration
	log  *logger.Logger
}

// NewSessionService 创建会话服务实例
func NewSessionService(repo Repository[*entity.Session], ttl time.Duration, log *logger.Logger) *SessionService {
	return &SessionService{
		repo: repo,
		ttl:  ttl,
		log:  log,
	}
}

// CreateSession 创建新会话
// 业务逻辑：创建新的会话实体并保存
func (s *SessionService) CreateSession(ctx context.Context, id string) (*entity.Session, error) {
	session := entity.NewSession(id, s.ttl)
	if err := s.repo.Save(ctx, session); err != nil {
		s.log.Error("创建会话失败: %v", err)
		return nil, err
	}
	return session, nil
}

// GetSession 获取会话
// 业务逻辑：获取会话并检查是否过期
func (s *SessionService) GetSession(ctx context.Context, id string) (*entity.Session, error) {
	session, err := s.repo.FindByID(ctx, id)
	if err != nil {
		s.log.Error("获取会话失败: %v", err)
		return nil, err
	}

	// 检查会话是否过期
	if session.IsExpired() {
		if err := s.repo.Delete(ctx, id); err != nil {
			s.log.Error("删除过期会话失败: %v", err)
			return nil, err
		}
		return nil, errs.ErrSessionNotFound
	}

	return session, nil
}

// SaveSession 保存会话
// 业务逻辑：保存会话数据，自动更新过期时间
func (s *SessionService) SaveSession(ctx context.Context, session *entity.Session) error {
	if session == nil {
		s.log.Error("保存会话失败: %v", errs.ErrInvalidSession)
		return errs.ErrInvalidSession
	}

	// 更新过期时间
	session.UpdateExpiry(s.ttl)

	return s.repo.Save(ctx, session)
}

// DeleteSession 删除会话
// 业务逻辑：永久删除会话
func (s *SessionService) DeleteSession(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		s.log.Error("删除会话失败: %v", err)
		return err
	}
	return nil
}

// UpdateSessionData 更新会话数据
// 业务逻辑：更新会话中的特定数据
func (s *SessionService) UpdateSessionData(ctx context.Context, id string, key string, value interface{}) error {
	session, err := s.GetSession(ctx, id)
	if err != nil {
		s.log.Error("更新会话数据失败: %v", err)
		return err
	}
	session.SetData(key, value)
	if err := s.SaveSession(ctx, session); err != nil {
		s.log.Error("保存会话数据失败: %v", err)
		return err
	}
	return nil
}

// GetSessionData 获取会话数据
// 业务逻辑：获取会话中的特定数据
func (s *SessionService) GetSessionData(ctx context.Context, id string, key string) (interface{}, bool, error) {
	session, err := s.GetSession(ctx, id)
	if err != nil {
		s.log.Error("获取会话数据失败: %v", err)
		return nil, false, err
	}

	value, exists := session.GetData(key)
	return value, exists, nil
}

// ClearExpiredSessions 清理过期会话
// 业务逻辑：批量清理过期会话（可选实现）
func (s *SessionService) ClearExpiredSessions(ctx context.Context) error {
	sessions, err := s.repo.FindAll(ctx)
	if err != nil {
		s.log.Error("清理过期会话失败: %v", err)
		return err
	}

	now := time.Now()
	for _, session := range sessions {
		if now.After(session.Expiry) {
			if err := s.repo.Delete(ctx, session.Id); err != nil {
				s.log.Error("删除过期会话失败: %v", err)
				return err
			}
		}
	}

	return nil
}
