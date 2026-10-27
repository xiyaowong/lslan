# lslan

Windows 局域网设备扫描工具。扫描网段内的主机，查看在线状态、设备类型、开放端口与网络连通性。

## 特性

- 主机扫描:ICMP / ARP / mDNS 多方式探测，展示 IP、MAC、厂商、主机名与 NetBIOS 名称
- 设备识别：基于 OUI 与 mDNS/UPnP 信息自动识别路由器、NAS、打印机等
- 端口扫描：常用端口开放状态，支持浏览 SMB 共享与 telnet 连接测试
- 网络诊断:Ping、Tracert，选中设备直接发起
- 远程唤醒:Wake-on-LAN，唤醒局域网内关机的设备
- 界面:Fluent 风格，深浅色主题跟随系统，支持手动切换
- 应用内自动更新

## 下载安装

从 [Releases](https://github.com/xiyaowong/lslan/releases) 下载：安装版 (Setup) 或便携版 (压缩包),解压后运行。

系统要求:Windows 64 位。

首次运行若 Windows 防火墙弹出提示，请选择允许。
