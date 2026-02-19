words=("login" "session" "oauth" "jwt" "token" "cache" "endpoint" "route" "middleware" "database" "encryption" "credential" "webhook" "permission" "mutex"           
   "goroutine" "handler" "config" "deploy" "pipeline" "secret" "password" "api" "http" "socket" "autotune" "bigram" "domain" "learner" "index" "search" "observe"
   "watcher" "security" "auth" "reindex" "rebuild" "parse" "tree" "cobra" "daemon" "status" "metric" "runway" "savings" "guided" "context" "prompt" "signal"
   "enricher")
   while true; do
     w="${words[$RANDOM % ${#words[@]}]}"                            
     ./aoa grep "$w" > /dev/null 2>&1
     sleep 1            
   done  
