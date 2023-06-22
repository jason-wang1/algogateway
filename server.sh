#!/usr/bin/bash
export LANG="zh_CN.UTF-8"

# 保证执行路径从项目目录开始
# '/*' 绝对路径
# '*'  表示任意字符串
case $0 in
/*)
    SCRIPT="$0"
    ;;
*)
    PWD=$(pwd)
    SCRIPT="$PWD/$0"
    ;;
esac
REALPATH=$(dirname $SCRIPT)
cd $REALPATH

project_config_file=./Release/project_config

package_name=$(cat ${project_config_file} | grep "package_name" | awk -F"=" '{print $2}')

build_folder_path=$(cat ${project_config_file} | grep "build_folder_path" | awk -F"=" '{print $2}')
build_exec_name=$(cat ${project_config_file} | grep "build_exec_name" | awk -F"=" '{print $2}')

version_file_path=$(cat ${project_config_file} | grep "version_file_path" | awk -F"=" '{print $2}')
version_key=$(cat ${project_config_file} | grep "version_key" | awk -F"=" '{print $2}')
version=$(cat ${version_file_path} | grep ${version_key} | awk -F '"' '{print $2}')

proxy_generate_path=$(cat ${project_config_file} | grep "proxy_generate_path" | awk -F"=" '{print $2}')
proxy_out_path=$(cat ${project_config_file} | grep "proxy_out_path" | awk -F"=" '{print $2}')

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

func_get_pid() {
    echo $(ps -x | grep ${1}_${3} | grep ${2} | grep -v 'grep')
}

func_start_server() {
    jq -r '.server_list[]|"\(.ip) \(.exe) \(.port) \(.nickname)"' config/server.json | while read ip exe port nickname; do
        if [ $ip = $local_ip ]; then
            if [ "$#" -ge 1 ] && [ ${exe} == "$1" ]; then
                sh ./exec_start_service.sh $exe $ip $port $nickname
            fi
            if [ "$#" -eq 0 ]; then
                sh ./exec_start_service.sh $exe $ip $port $nickname
            fi
        fi
    done
}

func_stop_server() {
    jq -r '.server_list[]|"\(.ip) \(.exe) \(.port)"' config/server.json | while read ip exe port; do
        if [ $ip = $local_ip ]; then
            if [ "$#" -ge 1 ] && [ ${exe} == "$1" ]; then
                sh ./exec_stop_service.sh $exe $ip $port
            fi
            if [ "$#" -eq 0 ]; then
                sh ./exec_stop_service.sh $exe $ip $port
            fi
        fi
    done
}

func_status_server() {
    jq -r '.server_list[]|"\(.ip) \(.exe) \(.port)"' config/server.json | while read ip exe port; do
        if [ $ip = $local_ip ]; then
            func_get_pid $exe $ip $port
        fi
    done
}

func_build() {
    # 通过参数决定编译项目/编译模式
    param_failed=true
    debug_build=true
    if [ "$#" -eq 1 ]; then
        if [ "$1" = "debug" ]; then
            debug_build=true
            param_failed=false
        elif [ "$1" = "release" ]; then
            debug_build=false
            param_failed=false
        fi
    fi

    if ${param_failed}; then
        echo "please input build_mode!!!"
        echo "  server.sh build debug"
        echo "  server.sh build release"
        exit 1
    fi

    echo "build ${build_exec_name} project"

    # go env
    set GOARCH=amd64
    set GOOS=linux
    export GO111MODULE=on
    export GOPROXY=https://goproxy.cn

    # build 生成在 GateWay 文件夹内
    cd ./${build_folder_path}
    swag init -d ./,../Common/  # 更新 swagger doc
    go build

    rm -rf ../Bin/${build_exec_name}
    rm -rf ../Release/${build_exec_name}

    # 将文件复制到Bin或Release目录内
    if ${debug_build}; then
        cp ./${build_folder_path} ../Bin/${build_exec_name}
    else
        cp ./${build_folder_path} ../Bin/${build_exec_name}
        mv ./${build_folder_path} ../Release/${build_exec_name}
    fi

    # 返回上级目录
    cd ../

    if ${debug_build}; then
        echo "debug build succ, version = "${version}
    else
        echo "release build succ, version = "${version}
    fi
}

func_clean() {
    rm -rf Bin/*.core
    rm -rf Bin/*.out
    rm -rf Bin/monitor/__pycache__
    rm -rf Bin/monitor/__init__.py
    rm -rf Bin/monitor/*.pyc
    rm -rf Bin/report_log
    rm -rf Bin/${build_exec_name}
    rm -rf Bin/Log${build_exec_name}/

    rm -rf Release/*.core
    rm -rf Release/*.out
    rm -rf Release/monitor/__pycache__
    rm -rf Release/monitor/__init__.py
    rm -rf Release/monitor/*.pyc
    rm -rf Release/report_log
    rm -rf Release/${build_exec_name}
    rm -rf Release/Log${build_exec_name}/

    rm -rf ${package_name}_*

    rm -rf Log${build_exec_name}
}

func_package() {
    # 编译
    func_clean
    func_build release

    # 创建临时目录, 将文件夹copy过去
    rm -rf ${package_name}_*
    folderName="${package_name}_${version}"
    cp -r ./Release ./${folderName}
    echo -e "\n# 版本信息\nservice_semver=${version}" >>./${folderName}/project_config
}

func_proxy() {
    ./${proxy_generate_path} ${proxy_out_path}
}

case "$1" in
'build')
    func_build $2
    ;;
'clean')
    func_clean
    ;;
'package')
    func_package $2
    ;;
'start')
    cd Bin
    func_start_server
    ;;
'stop')
    cd Bin
    func_stop_server
    ;;
'restart')
    cd Bin
    func_stop_server
    func_start_server
    ;;
'status')
    cd Bin
    func_status_server
    ;;
'proxy')
    func_proxy
    ;;
*)
    printf "action list: \n"
    printf "  help              -- 帮助菜单 \n"
    printf "  build             -- 编译项目 \n"
    printf "  clean             -- 清理项目 \n"
    printf "  status            -- 本机服务状态 \n"
    printf "  package           -- 打包项目 \n"
    printf "  proxy             -- 生成协议 \n"
    exit 1
    ;;
esac
