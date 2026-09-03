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

func (r *SubmissionRepo) Create(s *model.Submission) error {
	res, err := r.db.Exec(`
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
	return nil
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
