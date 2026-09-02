package service

import (
	"errors"
	"time"

	"blacklist-index/internal/model"
	"blacklist-index/internal/repository"
)

// AnnouncementService 负责公告的读取与保存。
type AnnouncementService struct {
	repo *repository.AnnouncementRepo
	loc  *time.Location
}

func NewAnnouncementService(repo *repository.AnnouncementRepo, loc *time.Location) *AnnouncementService {
	return &AnnouncementService{repo: repo, loc: loc}
}

// GetActiveHTML 获取生效公告渲染后的 HTML（无公告返回空字符串）。
func (s *AnnouncementService) GetActiveHTML() (string, error) {
	a, err := s.repo.GetActive()
	if errors.Is(err, repository.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return RenderMarkdown(a.Content), nil
}

// GetActive 获取生效公告原始内容（用于后台预填）。
func (s *AnnouncementService) GetActive() (*model.Announcement, error) {
	a, err := s.repo.GetActive()
	if errors.Is(err, repository.ErrNotFound) {
		return nil, nil
	}
	return a, err
}

// Save 保存公告内容。
func (s *AnnouncementService) Save(content, by string) error {
	a := &model.Announcement{
		Content:    content,
		ContentRaw: MarkdownToPlain(content),
		IsActive:   true,
		UpdatedAt:  NowStr(s.loc),
		UpdatedBy:  by,
	}
	return s.repo.Upsert(a)
}
