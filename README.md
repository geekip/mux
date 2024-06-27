# geekip/mux
A simple and lightweight Go HTTP router implemented using a trie tree.

## Features

* [Static routes](#static-routes)
* [Custom handler](#custom-handler)
* [Custom error handler](#custom-error-handler)
* [Methods](#methods)
* [Parameters](#parameters)
* [Wildcard](#wildcard)
* [Group](#group)
* [Middleware](#middleware)
* [FileServe](#file-serve)

# Install
`$ go get -u github.com/geekip/mux`

# Usage

### Static routes
``` go
func handler(w http.ResponseWriter, req *http.Request) {
  w.Write([]byte("hello world!"))
}

func main() {
  router := mux.New()
  router.Handle("/hello", http.HandlerFunc(handler))
  router.HandlerFunc("/world", handler)

  log.Fatal(http.ListenAndServe(":8080", router))
}
```

### Custom handler
``` go
type Handler struct{
  content string
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
  w.Write([]byte(h.content))
}

func main() {
  router := mux.New()
  router.Handle("/hello", &Handler{content: "Custom handler"})

  log.Fatal(http.ListenAndServe(":8080", router))
}
```

### Custom error handler

``` go
func handler(w http.ResponseWriter, req *http.Request) {
  w.Write([]byte("hello world!"))
}

func main() {
  router := mux.New()
  router.NotFound(func(w http.ResponseWriter, r *http.Request) {
    http.Error(w, "404 page not found", http.StatusNotFound)
  })
  router.InternalError(func(w http.ResponseWriter, r *http.Request, err interface{}) {
    http.Error(w, "500 internal server error", http.StatusInternalServerError)
  })
  router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
    http.Error(w, "405 method not allowed", http.StatusMethodNotAllowed)
  })

  router.HandlerFunc("/user", handler)
  log.Fatal(http.ListenAndServe(":8080", router))
}
```

### Methods

``` go
func handler(w http.ResponseWriter, req *http.Request) {
  w.Write([]byte("hello world!"))
}

func main() {
  router := mux.New()
  // all Methods
  router.Handle("/hello", http.HandlerFunc(handler))
  router.Method("*").Handle("/hello", http.HandlerFunc(handler))
  // GET
  router.Method("GET").Handle("/hello", http.HandlerFunc(handler))
  // More...
  router.Method("POST","PUT").Handle("/hello", http.HandlerFunc(handler))

  log.Fatal(http.ListenAndServe(":8080", router))
}
```

### Parameters

``` go
func handler(w http.ResponseWriter, req *http.Request) {
  params,_ := mux.Params(req)
  w.Write([]byte("match user/:id ! get id:" + params["id"]))
}

func main() {
  router := mux.New()
  // http://localhost:8080/user/123
  router.Handle("/user/:id", http.HandlerFunc(handler))
  
  log.Fatal(http.ListenAndServe(":8080", router))
}
```

### Wildcard

``` go
func handler(w http.ResponseWriter, req *http.Request) {
  params := mux.Params(req)
  // foo/bar
  w.Write([]byte(params["*"]))
}

func main() {
  router := mux.New()
  // http://localhost:8080/user/foo/bar
  router.Handle("/user/*", http.HandlerFunc(handler))
  
  log.Fatal(http.ListenAndServe(":8080", router))
}
```


### Group

``` go
func handler(w http.ResponseWriter, req *http.Request) {
  w.Write([]byte("hello world!"))
}

func main() {
  router := mux.New()
  user := router.Group("/admin")
  {
    // get /admin/user/list
    user.Method("GET").HandlerFunc("/user",handler)
    // put /admin/user/edit
    user.Method("PUT").HandlerFunc("/user",handler)
  }
  
  log.Fatal(http.ListenAndServe(":8080", router))
}
```

### Middleware

``` go
func handler(w http.ResponseWriter, req *http.Request) {
  w.Write([]byte("hello world!"))
}

func middleware1(next http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    ctx := context.WithValue(r.Context(), "user", "admin")
    w.Write([]byte("middleware 1"))
    next.ServeHTTP(w, r.WithContext(ctx))
  })
}

func middleware2(next http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if user, ok := r.Context().Value("user").(string); ok {
      w.Write([]byte("middleware 2, user:"+user))
    }
    next.ServeHTTP(w, r)
  })
}

func main() {
  router := mux.New()
  router.Use(middleware1, middleware2)
  router.HandlerFunc("/user", handler)
  
  log.Fatal(http.ListenAndServe(":8080", router))
}
```

### FileServe

``` go
func fileHandler(dir string) http.Handler {
  return func(w http.ResponseWriter, req *http.Request) {
    params := mux.Params(req)
    basePath := strings.TrimSuffix(req.URL.Path, params["*"])
    fs := http.StripPrefix(basePath, http.FileServer(http.Dir(dir)))
    fs.ServeHTTP(w, req)
  }
}

func main() {
  router := mux.New()
  router.HandleFunc("/files/*",fileHandler("./folder"))
  
  log.Fatal(http.ListenAndServe(":8080", router))
}
```