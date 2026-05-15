package ctxkey

// TraceID is the *gin.Context key for the request correlation id (see middleware.RequestID).
const TraceID = "trace_id"

// AdminID holds the authenticated admin UUID string (*gin.Context key).
const AdminID = "admin_id"
