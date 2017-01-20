
# maybe more powerful
# for mac (sed for linux is different)
grep "github.com/reechou/gorobot" * -R | grep -v Godeps | awk -F: '{print $1}' | sort | uniq | xargs sed -i '' 's#github.com/reechou/gorobot#github.com/reechou/wxrobot#g'
