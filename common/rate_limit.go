package common

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Cubeta de tokens de un cliente (IP). Se recarga de forma continua segun el tiempo transcurrido
type rateBucket struct {
	tokens   float64
	lastSeen time.Time
}

// Limitador en memoria: cada contenedor (prod, sandbox, qa, dev) es una sola instancia detras de Caddy,
// por eso no hace falta compartir los contadores en base de datos. Al reiniciar se pierden y es aceptable
type rateLimiter struct {
	mu              sync.Mutex
	buckets         map[string]*rateBucket
	capacity        float64
	refillPerSecond float64
	window          time.Duration
}

// RateLimitMiddleware limita las peticiones por IP con una cubeta de tokens: permite rafagas de hasta
// `requests` peticiones y recarga `requests` tokens cada `per`. Cada llamada crea su propio limitador,
// asi que las rutas que comparten la misma instancia comparten el limite. Debe ir primero en la cadena
// para que las peticiones rechazadas no lleguen a los validadores ni a MongoDB
func RateLimitMiddleware(requests int, per time.Duration) func(http.Handler) http.Handler {
	// Valores invalidos se corrigen para no dejar la ruta bloqueada ni sin limite
	if requests < 1 {
		requests = 1
	}
	if per <= 0 {
		per = time.Minute
	}

	limiter := &rateLimiter{
		buckets:         make(map[string]*rateBucket),
		capacity:        float64(requests),
		refillPerSecond: float64(requests) / per.Seconds(),
		window:          per,
	}
	go limiter.cleanup()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			res := NewResponseHandler(w)

			clientIP := ParseClientIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For"))
			allowed, retryAfterSeconds := limiter.allow(clientIP, time.Now())
			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
				res.Send(http.StatusTooManyRequests, "FAIL", "Demasiadas peticiones, intenta mas tarde", "CMMRLE001", map[string]any{"retryAfterSeconds": retryAfterSeconds})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// allow consume un token de la IP. Si no hay, devuelve los segundos que faltan para el siguiente token
func (l *rateLimiter) allow(key string, now time.Time) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	bucket, exists := l.buckets[key]
	if !exists {
		bucket = &rateBucket{tokens: l.capacity, lastSeen: now}
		l.buckets[key] = bucket
	}

	elapsedSeconds := now.Sub(bucket.lastSeen).Seconds()
	bucket.tokens = min(l.capacity, bucket.tokens+elapsedSeconds*l.refillPerSecond)
	bucket.lastSeen = now

	if bucket.tokens >= 1 {
		bucket.tokens--
		return true, 0
	}

	missingSeconds := (1 - bucket.tokens) / l.refillPerSecond
	retryAfterSeconds := max(1, int(math.Ceil(missingSeconds)))
	return false, retryAfterSeconds
}

// cleanup borra las cubetas sin uso para que el mapa no crezca sin limite. Una cubeta inactiva por una
// ventana completa ya esta llena, asi que borrarla no cambia el comportamiento
func (l *rateLimiter) cleanup() {
	ticker := time.NewTicker(l.window)
	defer ticker.Stop()

	for now := range ticker.C {
		l.mu.Lock()
		for key, bucket := range l.buckets {
			if now.Sub(bucket.lastSeen) >= l.window {
				delete(l.buckets, key)
			}
		}
		l.mu.Unlock()
	}
}
