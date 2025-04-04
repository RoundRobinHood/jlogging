# jlogging
Simple json logging and recovery middleware for [Gin](https://github.com/gin-gonic/gin).

## Setup
Get `jlogging`:
``` bash
go get github.com/RoundRobinHood/jlogging
```
Add middleware:
``` Go
// r := gin.New();
r.Use(jlogging.Middleware())
```

## Functionality
### Logging
`jlogging` prints a single log per request, and allows you to add details to it as the request passes through your code.
Adding details is done by accessing the `jlogging.RequestLog` object from your callback's Gin context. It can then be used to attach additional information to the request:
``` Go
func Login(c *gin.Context) {
    // Get request log
    jrl, exists := c.Get("jrl")
    if !exists {
        log.Printf("jlogging middleware absent.")
        return
    }
    l := jrl.(*jlogging.RequestLog)

    // Get submitted credentials
    var cred struct {
        Username string `json:"username"`
        Pwd      string `json:"password"`
    }
    if err := c.ShouldBindJson(&cred); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        l.Printf("Invalid JSON body submitted")
        l.Details["err"] = err
        return
    }

    // ...
```
### Recovery
In the event of a panic, jlogging recovers, sets `ResponseStatus` to 500, and the `error` value of `jlogging.RequestLog` to a panic details object documenting the panic, a stack trace, and the original status (before the panic occurred).
``` Go
type PanicDetails struct {
	Descriptor  any    `json:"desc"`
	PriorStatus int    `json:"oldStatus"`
	StackTrace  string `json:"stackTrace"`
}
```
This is then included in the object logged to stdout. Marshal failures are handled incrementally, with object details being removed until a last-resort string is printed with the format:
``` Go
fmt.Printf("{\"jlog\":\"Could not marshal request log during panic: %s\"}\n", err)
```
