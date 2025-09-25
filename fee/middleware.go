package fee

import (
	"encore.dev/middleware"
)

//encore:middleware target=tag:idempotency
func (s *Service) IdempotencyMiddleware(req middleware.Request, next middleware.Next) middleware.Response {
	// 1. Get the idempotency key from the request header.
	idempotencyKey := req.Data().Headers.Get("Idempotency-Key")

	// If no idempotency key is present, this middleware does nothing.
	if idempotencyKey == "" || req.Data().Method == "GET" {
		return next(req)
	}

	// 2. Check if the response is already cached for this idempotency key.
	//    This would involve a lookup in a cache like Redis.
	//    `cachedResponse, err := cache.Get(idempotencyKey)`
	//    If a cached response is found, return it immediately.
	//    `if cachedResponse != nil { return cachedResponse }`

	// 3. If this is a new request, lock the idempotency key to handle concurrent requests.
	//    This prevents race conditions where two identical requests are processed at the same time.
	//    `err := cache.Lock(idempotencyKey)`
	//    If locking fails, it means another request with the same key is already being processed.
	//    You might want to return an error (e.g., 409 Conflict) or wait for the lock to be released.
	//    `if err != nil { return middleware.Response{...} }`
	//    `defer cache.Unlock(idempotencyKey)`

	// 4. If we have the lock, proceed to call the actual API endpoint.
	resp := next(req)

	// 5. After the request is processed, cache the response if it was successful.
	//    A successful response is typically one with a 2xx status code.
	//    `if resp.StatusCode >= 200 && resp.StatusCode < 300 {`
	//    `  cache.Set(idempotencyKey, resp)`
	//    `}`

	// 6. Return the response to the client. The lock will be released by the defer statement.
	return resp
}
