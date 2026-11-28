package onlineconf

import (
	"hash/fnv"

	"github.com/google/uuid"

	contracts "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/contracts/onlineconf"
)

// bucketCount — число корзин раскатки: шаг процента 0,01 %.
const bucketCount = 10000

// Bucket возвращает корзину пользователя для флага: fnv64a(user_id + ":" + flag_key) % 10000.
// Одна и та же функция должна использоваться и в сервисе OnlineConf, и у потребителей,
// иначе админка и сервисы будут по-разному определять, включён ли флаг.
// Ключ флага берётся без скоупа, userID записывается в каноническом виде uuid.String().
func Bucket(userID uuid.UUID, flagKey string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(userID.String() + ":" + flagKey))
	return h.Sum64() % bucketCount
}

// Evaluate вычисляет флаг для пользователя.
// Порядок: аноним (uuid.Nil) всегда выключен, затем force_off, force_on и раскатка.
func Evaluate(f contracts.Flag, userID uuid.UUID) contracts.Decision {
	if userID == uuid.Nil {
		return contracts.Decision{Enabled: false, Reason: contracts.ReasonAnonymous}
	}
	if _, ok := f.ForceOff[userID]; ok {
		return contracts.Decision{Enabled: false, Reason: contracts.ReasonForceOff}
	}
	if _, ok := f.ForceOn[userID]; ok {
		return contracts.Decision{Enabled: true, Reason: contracts.ReasonForceOn}
	}

	percent := min(f.RolloutPercent, 100)
	if Bucket(userID, f.Key) < uint64(percent)*100 {
		return contracts.Decision{Enabled: true, Reason: contracts.ReasonRollout}
	}
	return contracts.Decision{Enabled: false, Reason: contracts.ReasonNotInRollout}
}
