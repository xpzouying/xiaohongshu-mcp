# 可选桌面层；MCP 二进制和内置浏览器继续来自官方镜像。
ARG MCP_IMAGE=xpzouying/xiaohongshu-mcp:v2.5.5
FROM ${MCP_IMAGE}
RUN apt-get update && apt-get install -y --no-install-recommends \
    xvfb x11vnc novnc websockify x11-utils openbox \
    fcitx5 fcitx5-rime rime-data-luna-pinyin fcitx5-frontend-gtk3 fcitx5-module-xorg \
    dbus-x11 xsettingsd \
    && apt-get clean && rm -rf /var/lib/apt/lists/*
COPY start.sh /usr/local/bin/xhs-novnc-start
COPY fcitx5-profile /usr/local/share/xhs-fcitx5-profile
COPY rime-default.custom.yaml /usr/local/share/xhs-rime-default.custom.yaml
RUN chmod 755 /usr/local/bin/xhs-novnc-start
ENV DISPLAY=:99 LANG=C.UTF-8 \
    XMODIFIERS=@im=fcitx GTK_IM_MODULE=fcitx QT_IM_MODULE=fcitx
EXPOSE 6080
ENTRYPOINT ["/usr/bin/tini", "-s", "-g", "--", "dbus-run-session", "--", "/usr/local/bin/xhs-novnc-start"]
CMD ["/app/app", "-headless=false"]
