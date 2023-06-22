#!/usr/bin/bash
export LANG="zh_CN.UTF-8"

project_config_file=./project_config
build_exec_name=$(cat ${project_config_file} | grep "build_exec_name" | awk -F"=" '{print $2}')

# 停止守护进程
./monitor.sh stop

# 杀死工作进程
sh kill.sh ${build_exec_name}
