package controller

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type redeemQuotaSyncCodeRequest struct {
	Code       string `json:"code"`
	RedeemedBy string `json:"redeemed_by"`
}

type debitQuotaSyncRequest struct {
	Amount     int    `json:"amount"`
	ExternalId string `json:"external_id"`
	Reason     string `json:"reason"`
}

func GenerateQuotaSyncCode(c *gin.Context) {
	userId := c.GetInt("id")
	code, row, err := model.CreateQuotaSyncCode(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"code":         code,
		"expires_time": row.ExpiresTime,
		"provider":     "YUAPI",
		"target":       "UAG 生图站",
		"purpose":      "YUAPI → UAG 生图站额度同步",
	})
}

func RedeemQuotaSyncCode(c *gin.Context) {
	var req redeemQuotaSyncCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	resp, err := model.RedeemQuotaSyncCode(req.Code, req.RedeemedBy)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, resp)
}

func GetQuotaSyncSnapshot(c *gin.Context) {
	token := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	if token == "" {
		token = strings.TrimSpace(c.Query("token"))
	}
	snapshot, err := model.GetQuotaSyncSnapshotByToken(token)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, snapshot)
}

func DebitQuotaSync(c *gin.Context) {
	token := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	var req debitQuotaSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	resp, err := model.DebitQuotaSyncByToken(token, req.Amount, req.ExternalId, req.Reason)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, resp)
}
