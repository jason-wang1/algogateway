#!/usr/bin/bash
export LANG="zh_CN.UTF-8"

if [ "$#" -le 0 ]; then
    echo "please input process name!"
    exit 1
fi

local_ip=$(
    python <<-EOF
import socket
import subprocess
s= socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
s.connect(('8.8.8.8', 80))
use_ip = s.getsockname()[0]
s.close()
print(use_ip)
EOF
)

username=$(whoami)
ServiceName=$1
pid=$(ps -u ${username} -f | grep -w ${ServiceName} | grep -w ${local_ip} | grep -v 'grep' | awk '{printf("%d ",$2);}')
if [ "$pid" != "" ]; then
    echo "-kill "${ServiceName}", pid = "${pid}
    kill ${pid}
fi
