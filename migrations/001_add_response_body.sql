-- 修改 logs 表，增加 response_body 字段并调整 user_input 字段类型

-- MySQL
ALTER TABLE logs
MODIFY COLUMN user_input MEDIUMTEXT COMMENT '用户输入内容(完整请求JSON)',
ADD COLUMN response_body MEDIUMTEXT COMMENT '模型响应内容(完整响应JSON)';

-- SQLite (如果需要支持)
-- ALTER TABLE logs ADD COLUMN response_body TEXT;
