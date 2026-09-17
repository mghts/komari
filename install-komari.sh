#!/bin/bash

# Color definitions for terminal output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "$1"
}

log_success() {
    echo -e "${GREEN}$1${NC}"
}

log_error() {
    echo -e "${RED}$1${NC}"
}

log_step() {
    echo -e "${YELLOW}$1${NC}"
}


# Global variables
INSTALL_DIR="/opt/komari"
DATA_DIR="/opt/komari"
SERVICE_NAME="komari"
BINARY_PATH="$INSTALL_DIR/komari"
BACKUP_DIR="$INSTALL_DIR/backup"
DATA_BACKUP_DIR="$DATA_DIR/data/backup"
DEFAULT_PORT="25774"
LISTEN_PORT=""
REPO="mghts/komari"
# fork 只安装用户明确选择的 Release（包括 RC），不使用上游 Snapshot/latest。
INSTALL_VERSION=""
# TUI 工具: whiptail / dialog / 空（回退纯文本）
TUI_TOOL=""

# ==========================================================
# TUI / 交互层
# ==========================================================

# 检测可用的 TUI 工具
detect_tui() {
    if command -v whiptail >/dev/null 2>&1; then
        TUI_TOOL="whiptail"
    elif command -v dialog >/dev/null 2>&1; then
        TUI_TOOL="dialog"
    else
        TUI_TOOL=""
    fi
}

# 是否启用 TUI
tui_enabled() {
    [ -n "$TUI_TOOL" ]
}

# 菜单选择
# 用法: ui_menu "标题" "提示" tag1 "item1" tag2 "item2" ...
# 返回: 选中的 tag（输出到 stdout），取消返回非零
ui_menu() {
    local title="$1"; shift
    local prompt="$1"; shift

    if tui_enabled; then
        $TUI_TOOL --title "$title" --menu "$prompt" 20 70 10 "$@" 3>&1 1>&2 2>&3
        return $?
    fi

    # 纯文本回退
    {
        echo
        echo "=============================================================="
        echo "  $title"
        echo "=============================================================="
        echo "$prompt"
        echo
        local tag item
        local args=("$@")
        local i=0
        while [ $i -lt ${#args[@]} ]; do
            tag="${args[$i]}"
            item="${args[$((i + 1))]}"
            echo "  $tag) $item"
            i=$((i + 2))
        done
        echo
    } >&2
    local choice
    read -r -p "输入选项: " choice >&2
    echo "$choice"
}

# 输入框
# 用法: ui_input "标题" "提示" "默认值"
# 返回: 输入内容（输出到 stdout），取消返回非零
ui_input() {
    local title="$1"
    local prompt="$2"
    local default="$3"

    if tui_enabled; then
        $TUI_TOOL --title "$title" --inputbox "$prompt" 12 70 "$default" 3>&1 1>&2 2>&3
        return $?
    fi

    local input
    read -r -p "$prompt [默认: $default]: " input >&2
    if [ -z "$input" ]; then
        echo "$default"
    else
        echo "$input"
    fi
}

# 是/否确认
# 用法: ui_yesno "标题" "提示"
# 返回: 0 表示 是，1 表示 否
ui_yesno() {
    local title="$1"
    local prompt="$2"

    if tui_enabled; then
        $TUI_TOOL --title "$title" --yesno "$prompt" 12 70
        return $?
    fi

    local confirm
    read -r -p "$prompt (Y/n): " confirm >&2
    if [[ $confirm =~ ^[Nn]$ ]]; then
        return 1
    fi
    return 0
}

# 信息提示框
# 用法: ui_msgbox "标题" "内容"
ui_msgbox() {
    local title="$1"
    local content="$2"

    if tui_enabled; then
        $TUI_TOOL --title "$title" --msgbox "$content" 20 72
        return
    fi

    echo
    echo "=============================================================="
    echo "  $title"
    echo "=============================================================="
    echo -e "$content"
    echo "=============================================================="
    read -r -p "按回车键继续..." _
}

# 显示横幅（仅纯文本模式）
show_banner() {
    if tui_enabled; then
        return
    fi
    clear
    echo "=============================================================="
    echo "            Komari Monitoring System Installer"
    echo "       https://github.com/mghts/komari"
    echo "=============================================================="
    echo
}

# 使用明确版本，避免尚无正式 Release 时 latest 指向不存在的资源。
select_version() {
    local version
    version=$(ui_input "选择 fork 版本" "输入 mghts/komari Release 版本（如 1.4.4-rc.2）：" "$INSTALL_VERSION") || return 1
    if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
        ui_msgbox "错误" "必须输入明确的 Release 版本，不能使用 latest 或 Snapshot。"
        return 1
    fi
    INSTALL_VERSION="$version"
}

# ==========================================================
# 基础检查
# ==========================================================

# Check if running as root
check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "请使用 root 权限运行此脚本"
        exit 1
    fi
}

# Check for systemd
check_systemd() {
    if ! command -v systemctl >/dev/null 2>&1; then
        return 1
    else
        return 0
    fi
}

# Detect system architecture
detect_arch() {
    [[ "$(uname -s)" = Linux ]] || { log_error "仅支持 Linux amd64/arm64" >&2; return 1; }
    local arch=$(uname -m)
    case $arch in
        x86_64)
            echo "amd64"
            ;;
        aarch64)
            echo "arm64"
            ;;
        *)
            log_error "不支持的架构: $arch"
            exit 1
            ;;
    esac
}

# Check if Komari is already installed
is_installed() {
    if [ -f "$BINARY_PATH" ]; then
        return 0 # 0 means true in bash exit codes
    else
        return 1 # 1 means false
    fi
}

# Install dependencies
install_dependencies() {
    log_step "检查并安装依赖..."

    if ! command -v curl >/dev/null 2>&1; then
        if command -v apt >/dev/null 2>&1; then
            log_info "使用 apt 安装依赖..."
            apt update
            apt install -y curl
        elif command -v yum >/dev/null 2>&1; then
            log_info "使用 yum 安装依赖..."
            yum install -y curl
        elif command -v apk >/dev/null 2>&1; then
            log_info "使用 apk 安装依赖..."
            apk add curl
        else
            log_error "未找到支持的包管理器 (apt/yum/apk)"
            exit 1
        fi
    fi
}

# 下载到独立保留目录，校验成功前不改动现有程序或服务。
download_verified_binary() {
    local arch="$1" stage filename base expected actual
    filename="komari-linux-${arch}"
    base="https://github.com/${REPO}/releases/download/${INSTALL_VERSION}"
    command -v sha256sum >/dev/null 2>&1 || { log_error "需要 sha256sum" >&2; return 1; }
    stage=$(mktemp -d "${TMPDIR:-/tmp}/komari-download.XXXXXXXX") || return 1
    log_info "下载文件保留在 $stage" >&2
    curl -fsSL "$base/$filename" -o "$stage/$filename" || return 1
    curl -fsSL "$base/SHA256SUMS" -o "$stage/SHA256SUMS" || return 1
    expected=$(awk -v name="$filename" '$2 == name {print $1}' "$stage/SHA256SUMS")
    [[ "$expected" =~ ^[0-9a-fA-F]{64}$ ]] || { log_error "SHA256SUMS 缺少唯一有效校验值" >&2; return 1; }
    actual=$(sha256sum "$stage/$filename") || return 1
    [[ "${actual%% *}" = "$expected" ]] || { log_error "SHA256 校验失败" >&2; return 1; }
    chmod +x "$stage/$filename" || return 1
    echo "$stage/$filename"
}

# ==========================================================
# 业务操作
# ==========================================================

# Binary installation
install_binary() {
    log_step "开始二进制安装..."

    if is_installed; then
        ui_msgbox "提示" "Komari 已安装。\n如需升级，请使用主菜单中的升级选项。"
        return
    fi

    select_version || return 1

    # 监听端口输入，校验范围 1-65535
    while true; do
        local input_port
        input_port=$(ui_input "监听端口" "请输入 Komari 的监听端口 (1-65535)：" "$DEFAULT_PORT")
        # 取消输入
        if [ $? -ne 0 ]; then
            log_info "安装已取消"
            return
        fi
        if [[ -z "$input_port" ]]; then
            LISTEN_PORT="$DEFAULT_PORT"
            break
        elif [[ "$input_port" =~ ^[0-9]+$ ]] && (( input_port >= 1 && input_port <= 65535 )); then
            LISTEN_PORT="$input_port"
            break
        else
            ui_msgbox "错误" "端口号无效，请输入 1-65535 之间的数字。"
        fi
    done

    install_dependencies

    local arch
    arch=$(detect_arch) || return 1
    log_info "检测到架构: $arch"

    log_step "创建安装目录: $INSTALL_DIR"
    mkdir -p "$INSTALL_DIR"

    log_step "创建数据目录: $DATA_DIR"
    mkdir -p "$DATA_DIR"

    local staged_binary
    staged_binary=$(download_verified_binary "$arch") || return 1
    cp "$staged_binary" "$BINARY_PATH" || return 1

    log_success "Komari 二进制文件安装完成: $BINARY_PATH"

    if ! check_systemd; then
        ui_msgbox "安装完成" "警告：未检测到 systemd，已跳过服务创建。\n\n您可以手动运行 Komari：\n    $BINARY_PATH server -l 0.0.0.0:$LISTEN_PORT"
        return
    fi

    create_systemd_service "$LISTEN_PORT"

    systemctl daemon-reload
    systemctl enable ${SERVICE_NAME}.service
    systemctl start ${SERVICE_NAME}.service

    if systemctl is-active --quiet ${SERVICE_NAME}.service; then
        log_success "Komari 服务启动成功"

        show_access_info "$LISTEN_PORT"
    else
        ui_msgbox "错误" "Komari 服务启动失败。\n\n查看日志: journalctl -u ${SERVICE_NAME} -f"
        return 1
    fi
}

# Create systemd service file
create_systemd_service() {
    local port="$1"
    log_step "创建 systemd 服务..."

    local service_file="/etc/systemd/system/${SERVICE_NAME}.service"
    cat > "$service_file" << EOF
[Unit]
Description=Komari Monitor Service
After=network.target

[Service]
Type=simple
ExecStart=${BINARY_PATH} server -l 0.0.0.0:${port}
WorkingDirectory=${DATA_DIR}
Restart=always
User=root

[Install]
WantedBy=multi-user.target
EOF

    log_success "systemd 服务文件创建完成"
}

# Show access information
show_access_info() {
    local port=${1:-$DEFAULT_PORT}
    local ip=$(hostname -I | awk '{print $1}')

    local content="安装完成！\n\n"
    content+="访问信息：\n"
    content+="  URL: http://${ip}:${port}\n"
    content+="\n首次使用请访问上述地址，按安装向导创建管理员账号。\n"
    content+="\n服务管理命令：\n"
    content+="  状态: systemctl status $SERVICE_NAME\n"
    content+="  启动: systemctl start $SERVICE_NAME\n"
    content+="  停止: systemctl stop $SERVICE_NAME\n"
    content+="  重启: systemctl restart $SERVICE_NAME\n"
    content+="  日志: journalctl -u $SERVICE_NAME -f"

    ui_msgbox "安装完成" "$content"
}

# Remove historical upgrade backups
cleanup_backups() {
    if ! ui_yesno "确认清理备份" "将删除 Komari 的二进制升级备份和数据升级压缩包。\n\n您确定要继续吗？"; then
        log_info "备份清理已取消"
        return 0
    fi

    log_step "清理升级历史备份..."
    rm -f -- "${BINARY_PATH}.backup."*

    local backup_dir
    for backup_dir in "$BACKUP_DIR" "$DATA_BACKUP_DIR"; do
        if [ -d "$backup_dir" ]; then
            find "$backup_dir" -maxdepth 1 -type f -name '*.zip' -delete
        fi
    done

    ui_msgbox "清理完成" "升级历史备份已清理。\n\n清理范围：\n  二进制备份: ${BINARY_PATH}.backup.*\n  数据压缩包: $BACKUP_DIR\n  数据压缩包: $DATA_BACKUP_DIR"
}

# Upgrade function
upgrade_komari() {
    log_step "升级 Komari..."

    if ! is_installed; then
        ui_msgbox "错误" "Komari 未安装。请先安装它。"
        return 1
    fi

    if ! check_systemd; then
        ui_msgbox "错误" "未检测到 systemd。无法管理服务。"
        return 1
    fi

    select_version || return 1

    install_dependencies
    local arch staged_binary backup_path
    arch=$(detect_arch) || return 1
    staged_binary=$(download_verified_binary "$arch") || return 1

    # 下载或校验失败不会停止旧服务；保留每次升级的备份。
    backup_path=$(mktemp "${BINARY_PATH}.backup.XXXXXXXX") || return 1
    cp -p "$BINARY_PATH" "$backup_path" || return 1
    systemctl stop "${SERVICE_NAME}.service" || return 1
    if cp "$staged_binary" "$BINARY_PATH" &&
       systemctl start "${SERVICE_NAME}.service" &&
       systemctl is-active --quiet "${SERVICE_NAME}.service"; then
        ui_msgbox "升级完成" "Komari 升级成功 (版本: $INSTALL_VERSION)。备份: $backup_path"
    else
        log_error "升级失败，尝试恢复旧二进制文件"
        systemctl stop "${SERVICE_NAME}.service" || return 1
        cp -p "$backup_path" "$BINARY_PATH" || return 1
        systemctl start "${SERVICE_NAME}.service" || return 1
        ui_msgbox "错误" "升级失败，已恢复旧二进制文件。请检查日志和数据兼容性。备份: $backup_path"
        return 1
    fi
}

# Uninstall function
uninstall_komari() {
    log_step "卸载 Komari..."

    if ! is_installed; then
        ui_msgbox "提示" "Komari 未安装。"
        return 0
    fi

    if ! ui_yesno "确认卸载" "这将删除 Komari 二进制文件和服务。\n\n您确定要继续吗？"; then
        log_info "卸载已取消"
        return 0
    fi

    if check_systemd; then
        log_step "停止并禁用服务..."
        systemctl stop ${SERVICE_NAME}.service >/dev/null 2>&1
        systemctl disable ${SERVICE_NAME}.service >/dev/null 2>&1
        rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
        systemctl daemon-reload
        log_success "systemd 服务已删除"
    fi

    log_step "删除二进制文件..."
    rm -f "$BINARY_PATH"
    # 尝试在目录为空时删除该目录
    rmdir "$INSTALL_DIR" 2>/dev/null || log_info "数据目录 $INSTALL_DIR 不为空，未删除"
    log_success "Komari 二进制文件已删除"

    ui_msgbox "卸载完成" "Komari 卸载完成。\n\n数据文件保留在 $DATA_DIR"
}

# Show service status
show_status() {
    if ! is_installed; then
        ui_msgbox "错误" "Komari 未安装。"
        return
    fi
    if ! check_systemd; then
        ui_msgbox "错误" "未检测到 systemd。无法获取服务状态。"
        return
    fi
    if tui_enabled; then
        local status_output
        status_output=$(systemctl status ${SERVICE_NAME}.service --no-pager -l 2>&1)
        ui_msgbox "服务状态" "$status_output"
    else
        log_step "Komari 服务状态:"
        systemctl status ${SERVICE_NAME}.service --no-pager -l
        read -r -p "按回车键继续..." _
    fi
}

# Show service logs
show_logs() {
    if ! is_installed; then
        ui_msgbox "错误" "Komari 未安装。"
        return
    fi
    if ! check_systemd; then
        ui_msgbox "错误" "未检测到 systemd。无法获取服务日志。"
        return
    fi
    # 日志为实时流，直接在终端显示
    if tui_enabled; then
        clear
    fi
    log_step "查看 Komari 服务日志 (按 Ctrl+C 退出)..."
    journalctl -u ${SERVICE_NAME} -f --no-pager
}

# Restart service
restart_service() {
    if ! is_installed; then
        ui_msgbox "错误" "Komari 未安装。"
        return
    fi
    if ! check_systemd; then
        ui_msgbox "错误" "未检测到 systemd。无法重启服务。"
        return
    fi
    log_step "重启 Komari 服务..."
    systemctl restart ${SERVICE_NAME}.service
    if systemctl is-active --quiet ${SERVICE_NAME}.service; then
        ui_msgbox "成功" "服务重启成功。"
    else
        ui_msgbox "错误" "服务重启失败，请检查日志。"
    fi
}

# Stop service
stop_service() {
    if ! is_installed; then
        ui_msgbox "错误" "Komari 未安装。"
        return
    fi
    if ! check_systemd; then
        ui_msgbox "错误" "未检测到 systemd。无法停止服务。"
        return
    fi
    log_step "停止 Komari 服务..."
    systemctl stop ${SERVICE_NAME}.service
    ui_msgbox "成功" "服务已停止。"
}


# Main menu
main_menu() {
    while true; do
        show_banner

        local choice
        choice=$(ui_menu "Komari 监控系统安装器" "请选择操作：" \
            "1" "安装 Komari" \
            "2" "升级 Komari" \
            "3" "卸载 Komari" \
            "4" "查看状态" \
            "5" "查看日志" \
            "6" "重启服务" \
            "7" "停止服务" \
            "8" "清理升级历史备份" \
            "9" "退出")

        # 用户在 TUI 中取消（ESC/Cancel）则退出
        if [ $? -ne 0 ] && tui_enabled; then
            clear
            exit 0
        fi

        case $choice in
            1) install_binary ;;
            2) upgrade_komari ;;
            3) uninstall_komari ;;
            4) show_status ;;
            5) show_logs ;;
            6) restart_service ;;
            7) stop_service ;;
            8) cleanup_backups ;;
            9)
                tui_enabled && clear
                exit 0 
                ;;
            *) ui_msgbox "错误" "无效选项" ;;
        esac

        # 纯文本模式下单次执行后退出循环（保持原有行为，避免输出被覆盖）
        if ! tui_enabled; then
            break
        fi
    done
}

# Main execution (sourcing defines functions for isolated installer tests).
if [[ -z "${BASH_SOURCE[0]:-}" || "${BASH_SOURCE[0]}" = "$0" ]]; then
    check_root
    detect_tui
    main_menu
fi
