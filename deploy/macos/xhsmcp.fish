function xhsmcp_stop
    launchctl stop xhsmcp
end

function xhsmcp_start
    launchctl start xhsmcp
end

function xhsmcp_status
    gomcp
    set service_name "xhsmcp"
    
    # 获取服务状态
    set pid_status (launchctl list | grep $service_name | awk '{print $1}')
    
    if test "$pid_status" != "-"
        echo "✓ $service_name 正在运行 (PID: $pid_status)"
        read -P "是否停止服务? (yes/其他): " answer
        if test "$answer" = "yes"
            xhsmcp_stop
            echo "✓ 服务已停止"
        else
            echo "取消操作"
        end
    else
        echo "✗ $service_name 未运行"
        read -P "是否启动服务? (yes/其他): " answer
        if test "$answer" = "yes"
            xhsmcp_start
            sleep 1
            set pid_status (launchctl list | grep $service_name | awk '{print $1}')
            if test "$pid_status" != "-"
                echo "✓ 服务启动成功 (PID: $pid_status)"
            else
                echo "✗ 服务启动失败，检查日志: /tmp/xhsmcp.err"
                return 1
            end
        else
            echo "取消操作"
            return 1
        end
    end
end