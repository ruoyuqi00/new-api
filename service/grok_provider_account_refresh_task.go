package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/bytedance/gopkg/util/gopool"
)

const (
	grokProviderAccountRefreshTick      = 10 * time.Minute
	grokProviderAccountRefreshThreshold = 30 * time.Minute
	grokProviderAccountRefreshTimeout   = 20 * time.Second
	grokProviderAccountRefreshBatchSize = 200
)

var (
	grokProviderAccountRefreshOnce    sync.Once
	grokProviderAccountRefreshRunning atomic.Bool
)

func StartGrokProviderAccountCredentialAutoRefreshTask() {
	grokProviderAccountRefreshOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		gopool.Go(func() {
			runGrokProviderAccountCredentialRefreshOnce()
			ticker := time.NewTicker(grokProviderAccountRefreshTick)
			defer ticker.Stop()
			for range ticker.C {
				runGrokProviderAccountCredentialRefreshOnce()
			}
		})
	})
}

func runGrokProviderAccountCredentialRefreshOnce() {
	if !grokProviderAccountRefreshRunning.CompareAndSwap(false, true) {
		return
	}
	defer grokProviderAccountRefreshRunning.Store(false)

	now := time.Now()
	refreshed := 0
	for offset := 0; ; offset += grokProviderAccountRefreshBatchSize {
		var accounts []model.ProviderAccount
		err := model.DB.
			Table("provider_accounts").
			Select("provider_accounts.*").
			Joins("JOIN account_pools ON account_pools.id = provider_accounts.pool_id").
			Where("account_pools.adapter_type = ? AND provider_accounts.type = ? AND provider_accounts.status = ?",
				constant.ChannelTypeXai, "oauth_json", model.ProviderAccountEnabled).
			Order("provider_accounts.id ASC").
			Limit(grokProviderAccountRefreshBatchSize).
			Offset(offset).
			Find(&accounts).Error
		if err != nil {
			logger.LogError(context.Background(), fmt.Sprintf("Grok provider account refresh query failed: %v", err))
			return
		}
		if len(accounts) == 0 {
			break
		}
		for _, account := range accounts {
			if account.ExpiresAt <= 0 || time.Unix(account.ExpiresAt, 0).Sub(now) > grokProviderAccountRefreshThreshold {
				continue
			}
			refreshCtx, cancel := context.WithTimeout(context.Background(), grokProviderAccountRefreshTimeout)
			credential, expiresAt, refreshErr := RefreshGrokOAuthCredential(refreshCtx, account.Credential)
			cancel()
			if refreshErr != nil {
				logger.LogWarn(context.Background(), fmt.Sprintf("Grok provider account refresh failed: account_id=%d err=%v", account.Id, refreshErr))
				continue
			}
			if err := model.UpdateProviderAccountCredential(account.Id, credential, expiresAt); err != nil {
				logger.LogWarn(context.Background(), fmt.Sprintf("Grok provider account credential save failed: account_id=%d err=%v", account.Id, err))
				continue
			}
			refreshed++
		}
		if len(accounts) < grokProviderAccountRefreshBatchSize {
			break
		}
	}
	if refreshed > 0 {
		model.InitAccountPoolCache()
		logger.LogInfo(context.Background(), fmt.Sprintf("Grok provider account refresh completed: refreshed=%d", refreshed))
	}
}
