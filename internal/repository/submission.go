package repository

import (
	"database/sql"
	"blacklist-index/internal/model"
)

type SubmissionRepo struct {
	db *sql.DB
}

func NewSubmissionRepo(db *sql.DB) *SubmissionRepo {
	return &SubmissionRepo{db: db}
}

func (r *SubmissionRepo) Create(s *model.Submission, images []*model.SubmissionImage) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`
		INSERT INTO submissions (query_code, type, email, ban_reason, event_link, event_related_people,
			appeal_reason, appeal_evidence, status, reject_reason, submitter_ip, submitter_ua, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.QueryCode, s.Type, s.Email, s.BanReason, s.EventLink, s.EventRelatedPeople,
		s.AppealReason, s.AppealEvidence, s.Status, s.RejectReason, s.SubmitterIP, s.SubmitterUA, s.CreatedAt)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	s.ID = id

	for i, img := range images {
		if _, err := tx.Exec(`
			INSERT INTO submission_images (submission_id, file_hash, ext, size, sort_order)
			VALUES (?, ?, ?, ?, ?)
		`, id, img.FileHash, img.Ext, img.Size, i); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ListImages 返回某提交关联的图片元数据，按 sort_order 升序。
func (r *SubmissionRepo) ListImages(submissionID int64) ([]*model.SubmissionImage, error) {
	rows, err := r.db.Query(`
		SELECT id, submission_id, file_hash, ext, size, sort_order
		FROM submission_images
		WHERE submission_id = ?
		ORDER BY sort_order ASC, id ASC
	`, submissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	images := make([]*model.SubmissionImage, 0)
	for rows.Next() {
		var img model.SubmissionImage
		if err := rows.Scan(&img.ID, &img.SubmissionID, &img.FileHash, &img.Ext, &img.Size, &img.SortOrder); err != nil {
			return nil, err
		}
		images = append(images, &img)
	}
	return images, rows.Err()
}

// ListReferencedImages 返回当前可达（仍需保留）的图片文件键（file_hash+"."+ext），用于清理不可达文件。
// 可达判定：图片属于「待审核」申请，或属于仍存在于黑名单（含回收站）中的记录。
// 已通过但黑名单被彻底删除、或已驳回的申请，其图片视为不可达，可在清理时删除。
func (r *SubmissionRepo) ListReferencedImages() (map[string]bool, error) {
	rows, err := r.db.Query(`
		SELECT DISTINCT si.file_hash, si.ext
		FROM submission_images si
		WHERE si.submission_id IN (SELECT id FROM submissions WHERE status = 'pending')
		   OR si.submission_id IN (SELECT submission_id FROM blacklist WHERE submission_id > 0)
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	set := make(map[string]bool)
	for rows.Next() {
		var h, e string
		if err := rows.Scan(&h, &e); err != nil {
			return nil, err
		}
		set[h+"."+e] = true
	}
	return set, rows.Err()
}

func (r *SubmissionRepo) GetByID(id int64) (*model.Submission, error) {
	row := r.db.QueryRow(`
		SELECT id, query_code, type, email, ban_reason, event_link, event_related_people,
			appeal_reason, appeal_evidence, status, reject_reason, submitter_ip, submitter_ua,
			created_at, reviewed_at, reviewed_by
		FROM submissions WHERE id = ?
	`, id)
	return scanSubmission(row)
}

func (r *SubmissionRepo) GetByQueryCode(code string) (*model.Submission, error) {
	row := r.db.QueryRow(`
		SELECT id, query_code, type, email, ban_reason, event_link, event_related_people,
			appeal_reason, appeal_evidence, status, reject_reason, submitter_ip, submitter_ua,
			created_at, reviewed_at, reviewed_by
		FROM submissions WHERE query_code = ?
	`, code)
	return scanSubmission(row)
}

func scanSubmission(s scanner) (*model.Submission, error) {
	var sub model.Submission
	var reviewedAt sql.NullString
	var reviewedBy sql.NullString
	err := s.Scan(&sub.ID, &sub.QueryCode, &sub.Type, &sub.Email, &sub.BanReason,
		&sub.EventLink, &sub.EventRelatedPeople, &sub.AppealReason, &sub.AppealEvidence,
		&sub.Status, &sub.RejectReason, &sub.SubmitterIP, &sub.SubmitterUA,
		&sub.CreatedAt, &reviewedAt, &reviewedBy)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if reviewedAt.Valid {
		sub.ReviewedAt = reviewedAt.String
	}
	if reviewedBy.Valid {
		sub.ReviewedBy = reviewedBy.String
	}
	return &sub, nil
}

func (r *SubmissionRepo) List(status string, typ string, page int, pageSize int) ([]*model.Submission, int64, error) {
	offset := (page - 1) * pageSize
	where := "WHERE 1=1"
	args := []any{}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	if typ != "" {
		where += " AND type = ?"
		args = append(args, typ)
	}

	var total int64
	err := r.db.QueryRow("SELECT COUNT(*) FROM submissions "+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(`
		SELECT id, query_code, type, email, ban_reason, event_link, event_related_people,
			appeal_reason, appeal_evidence, status, reject_reason, submitter_ip, submitter_ua,
			created_at, reviewed_at, reviewed_by
		FROM submissions `+where+`
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]*model.Submission, 0)
	for rows.Next() {
		s, err := scanSubmission(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, s)
	}
	return list, total, rows.Err()
}

func (r *SubmissionRepo) Approve(id int64, reviewedBy string, reviewedAt string) error {
	res, err := r.db.Exec(`
		UPDATE submissions SET status = 'approved', reviewed_by = ?, reviewed_at = ? WHERE id = ? AND status = 'pending'
	`, reviewedBy, reviewedAt, id)
	return checkAffected(res, err)
}

func (r *SubmissionRepo) Reject(id int64, reason string, reviewedBy string, reviewedAt string) error {
	res, err := r.db.Exec(`
		UPDATE submissions SET status = 'rejected', reject_reason = ?, reviewed_by = ?, reviewed_at = ? WHERE id = ? AND status = 'pending'
	`, reason, reviewedBy, reviewedAt, id)
	return checkAffected(res, err)
}
