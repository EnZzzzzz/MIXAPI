# 使用alpine作为基础镜像
FROM alpine:latest

#设置工作目录
WORKDIR /app

# 从本地复制二进制文件
COPY mixapi /app/mixapi

# 设置文件可执行权限
RUN chmod +x /app/mixapi

# 暴露3000端口
EXPOSE 3000

#设置工作目录
WORKDIR /data

# 启动命令
CMD ["/app/mixapi"]

