#!/bin/bash

# 初始化变量
SPECIFIED_REGION=""
AUTH_CODE=""

# 解析参数
while [[ $# -gt 0 ]]; do
    key="$1"
    case $key in
        -server)
        SPECIFIED_REGION="$2"
        shift # 移到下一个参数
        shift # 移到下一个参数值
        ;;
        -auth_code)
        AUTH_CODE="$2"
        shift
        shift
        ;;
        *)
        echo "未知的参数: $key"
        exit 1
        ;;
    esac
done

# 检查是否提供了必须的参数
if [ -z "$SPECIFIED_REGION" ] || [ -z "$AUTH_CODE" ]; then
    echo "缺少必要参数 -server 或 -auth_code"
    exit 1
fi

# 检查cloud-guard-agent服务状态
if systemctl is-active --quiet cloud-guard-agent; then
    echo "cloud-guard-agent正在运行，开始卸载..."
    # 卸载cloud-guard-agent软件
    sudo yum remove -y cloud-guard-agent
    if [ $? -eq 0 ]; then
        echo "cloud-guard-agent已成功卸载。"
    else
        echo "卸载cloud-guard-agent时出错。"
        exit 1
    fi
else
    echo "cloud-guard-agent未运行，无需卸载。"
fi

# 设置SPECIFIED_REGION为环境变量
export SPECIFIED_REGION

RPM_PACKAGE="cloud-guard-agent-1.0.2-1.x86_64.rpm"
# 安装RPM包
echo "正在安装 $RPM_PACKAGE..."
sudo rpm -ivh $RPM_PACKAGE
# 检查安装是否成功
if rpm -q cloud-guard-agent > /dev/null 2>&1; then
    echo "cloud-guard-agent 安装成功。"
else
    echo "cloud-guard-agent 安装失败。"
    exit 1
fi

# 将AUTH_CODE写入指定文件
CODE_FILE="/etc/cloud-guard/specified_code"
# 确保/etc/cloud-guard目录存在
if [ ! -d "/etc/cloud-guard" ]; then
    sudo mkdir -p /etc/cloud-guard
fi
# 写入auth_code并设置权限
echo "$AUTH_CODE" | sudo tee "$CODE_FILE" > /dev/null
sudo chmod 0600 "$CODE_FILE"
echo "SPECIFIED_REGION 环境变量设置为: $SPECIFIED_REGION"
echo "auth_code 已写入到 $CODE_FILE"

sudo systemctl start cloud-guard-agent
