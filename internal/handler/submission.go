package handler

import (
	"net/http"
)

// —— 公开接口 ——

type submitReportRequest struct {
	Email              string `json:"email"`
	BanReason          string `json:"ban_reason"`
	EventLink          string `json:"event_link"`
	EventRelatedPeople string `json:"event_related_people"`
}

// SubmitReport 用户提交举报（标记为不可信申请）。
func (h *Handler) SubmitReport(w http.ResponseWriter, r *http.Request) {
	var req submitReportRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	code, err := h.submission.SubmitReport(
		req.Email, req.BanReason, req.EventLink, req.EventRelatedPeople,
		clientIP(r), r.UserAgent(),
	)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"query_code": code})
}

type submitAppealRequest struct {
	Email          string `json:"email"`
	AppealReason   string `json:"appeal_reason"`
	AppealEvidence string `json:"appeal_evidence"`
}

// SubmitAppeal 用户提交申诉。
func (h *Handler) SubmitAppeal(w http.ResponseWriter, r *http.Request) {
	var req submitAppealRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	code, err := h.submission.SubmitAppeal(
		req.Email, req.AppealReason, req.AppealEvidence,
		clientIP(r), r.UserAgent(),
	)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"query_code": code})
}

// QuerySubmission 用户按查询码查询申请状态。
func (h *Handler) QuerySubmission(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "message": "请输入查询码"})
		return
	}
	sub, err := h.submission.GetByQueryCode(code)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{
		"id":              sub.ID,
		"type":            sub.Type,
		"email":           sub.Email,
		"status":          sub.Status,
		"ban_reason":      sub.BanReason,
		"event_link":      sub.EventLink,
		"related_people":  sub.EventRelatedPeople,
		"appeal_reason":   sub.AppealReason,
		"appeal_evidence": sub.AppealEvidence,
		"reject_reason":   sub.RejectReason,
		"created_at":      sub.CreatedAt,
		"reviewed_at":     sub.ReviewedAt,
	})
}

// —— 管理员接口 ——

// ListSubmissions 分页列出申请（后台审核页）。
func (h *Handler) ListSubmissions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := q.Get("status")
	typ := q.Get("type")
	page := intParam(q.Get("page"), 1)
	pageSize := clampInt(intParam(q.Get("page_size"), 20), 1, 100)

	list, total, err := h.submission.List(status, typ, page, pageSize)
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"total": total, "list": list, "page": page, "page_size": pageSize})
}

type rejectRequest struct {
	Reason string `json:"reason"`
}

// ApproveSubmission 通过申请。
func (h *Handler) ApproveSubmission(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	sub, err := h.submission.Approve(id, h.currentUser(r), clientIP(r), r.UserAgent())
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"submission": sub})
}

// RejectSubmission 驳回申请。
func (h *Handler) RejectSubmission(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req rejectRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	sub, err := h.submission.Reject(id, req.Reason, h.currentUser(r), clientIP(r), r.UserAgent())
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"submission": sub})
}
