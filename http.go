http.HandleFunc("/readiness", func(w http.ResponseWriter, r *http.Request) {
  ok := true
  errMsg = ""

  // Check memcache
  if mc != nil {
    err := mc.Set(&memcache.Item{Key: "healthz", Value: []byte("test")})
  }
  if mc == nil || err != nil {
    ok = false
    errMsg += "Memcached not ok.¥n"
  }

  // Check database
  if db != nil {
    _, err := db.Query("SELECT 1;")
  }
  if db == nil || err != nil {
    ok = false
    errMsg += "Database not ok.¥n"
  } 

  if ok {
    w.Write([]byte("OK"))
  } else {
    // Send 503
    http.Error(w, errMsg, http.StatusServiceUnavailable)
  }
})
http.ListenAndServe(":8080", nil)
