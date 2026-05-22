import React, { useEffect, useState, useRef, useCallback } from 'react';
import {
  Button,
  Col,
  Form,
  Row,
  Spin,
  DatePicker,
  Radio,
  Modal,
  Space,
  Typography,
  Table,
  Tag,
  Tooltip,
  Popconfirm,
  InputNumber,
} from '@douyinfe/semi-ui';
import {
  Download,
  Trash2,
  Loader,
} from 'lucide-react';
import dayjs from 'dayjs';
import { useTranslation } from 'react-i18next';
import {
  compareObjects,
  API,
  showError,
  showSuccess,
  showWarning,
  timestamp2string,
} from '../../../helpers';

const { Text } = Typography;

export default function SettingsLog(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [loadingCleanHistoryLog, setLoadingCleanHistoryLog] = useState(false);
  const [loadingExportLog, setLoadingExportLog] = useState(false);
  const [inputs, setInputs] = useState({
    LogConsumeEnabled: false,
  });
  const [exportInputs, setExportInputs] = useState({
    startTimestamp: dayjs().subtract(1, 'week').toDate(),
    endTimestamp: dayjs().endOf('day').toDate(),
    format: 'json',
    username: '',
    modelName: '',
    bodyExportMode: 'full',
    bodyExportLength: 500,
  });
  const [cleanInputs, setCleanInputs] = useState({
    startTimestamp: dayjs().subtract(1, 'month').startOf('day').toDate(),
    endTimestamp: dayjs().endOf('day').toDate(),
    cleanMode: 'all',
  });
  const refForm = useRef();
  const exportFormRef = useRef();
  const cleanFormRef = useRef();
  const [inputsRow, setInputsRow] = useState(inputs);

  // Export task list states
  const [exportTasks, setExportTasks] = useState([]);
  const [loadingTasks, setLoadingTasks] = useState(false);
  const pollTimerRef = useRef(null);

  const statusMap = {
    pending: { label: t('待处理'), color: 'grey' },
    processing: { label: t('处理中'), color: 'blue' },
    success: { label: t('已完成'), color: 'green' },
    failed: { label: t('失败'), color: 'red' },
  };

  const fetchExportTasks = useCallback(async () => {
    try {
      setLoadingTasks(true);
      const res = await API.get('/api/log/export/tasks?page=1&size=10');
      const { success, data } = res.data;
      if (success && data) {
        setExportTasks(data.items || []);
      }
    } catch (error) {
      console.error('获取导出任务列表失败', error);
    } finally {
      setLoadingTasks(false);
    }
  }, []);

  const startPolling = useCallback(() => {
    if (pollTimerRef.current) {
      clearInterval(pollTimerRef.current);
    }
    pollTimerRef.current = setInterval(() => {
      fetchExportTasks();
    }, 5000);
  }, [fetchExportTasks]);

  const stopPolling = useCallback(() => {
    if (pollTimerRef.current) {
      clearInterval(pollTimerRef.current);
      pollTimerRef.current = null;
    }
  }, []);

  useEffect(() => {
    fetchExportTasks();
  }, [fetchExportTasks]);

  useEffect(() => {
    const hasActiveTasks = exportTasks.some(
      (task) => task.status === 'pending' || task.status === 'processing'
    );
    if (hasActiveTasks) {
      startPolling();
    } else {
      stopPolling();
    }
    return () => stopPolling();
  }, [exportTasks, startPolling, stopPolling]);

  function onSubmit() {
    const updateArray = compareObjects(inputs, inputsRow).filter(
      (item) => item.key !== 'startTimestamp' && item.key !== 'endTimestamp' && item.key !== 'cleanMode',
    );

    if (!updateArray.length) return showWarning(t('你似乎并没有修改什么'));
    const requestQueue = updateArray.map((item) => {
      let value = '';
      if (typeof inputs[item.key] === 'boolean') {
        value = String(inputs[item.key]);
      } else {
        value = inputs[item.key];
      }
      return API.put('/api/option/', {
        key: item.key,
        value,
      });
    });
    setLoading(true);
    Promise.all(requestQueue)
      .then((res) => {
        if (requestQueue.length === 1) {
          if (res.includes(undefined)) return;
        } else if (requestQueue.length > 1) {
          if (res.includes(undefined))
            return showError(t('部分保存失败，请重试'));
        }
        showSuccess(t('保存成功'));
        props.refresh();
      })
      .catch(() => {
        showError(t('保存失败，请重试'));
      })
      .finally(() => {
        setLoading(false);
      });
  }

  async function onExportLog() {
    try {
      setLoadingExportLog(true);
      if (!exportInputs.startTimestamp || !exportInputs.endTimestamp) {
        throw new Error(t('请选择开始时间和结束时间'));
      }

      const startTimestamp = Math.floor(Date.parse(exportInputs.startTimestamp) / 1000);
      const endTimestamp = Math.floor(Date.parse(exportInputs.endTimestamp) / 1000);

      if (startTimestamp >= endTimestamp) {
        throw new Error(t('开始时间必须早于结束时间'));
      }

      const payload = {
        start_timestamp: startTimestamp,
        end_timestamp: endTimestamp,
        format: exportInputs.format,
        username: exportInputs.username || '',
        model_name: exportInputs.modelName || '',
        body_export_mode: exportInputs.bodyExportMode || 'full',
        body_export_length: parseInt(exportInputs.bodyExportLength, 10) || 500,
      };

      const res = await API.post('/api/log/export', payload);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('导出任务已创建，请在下方列表中查看'));
        await fetchExportTasks();
      } else {
        throw new Error(message || t('导出任务创建失败'));
      }
    } catch (error) {
      showError(error.message || t('日志导出失败'));
    } finally {
      setLoadingExportLog(false);
    }
  }

  async function onDownloadExportTask(task) {
    try {
      const response = await API.get(`/api/log/export/${task.id}/download`, {
        responseType: 'blob',
      });

      const blob = new Blob([response.data]);
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;

      const startStr = task.start_time ? timestamp2string(task.start_time).replace(/[-: ]/g, '').slice(0, 8) : '';
      const endStr = task.end_time ? timestamp2string(task.end_time).replace(/[-: ]/g, '').slice(0, 8) : '';
      const extension = task.format === 'csv' ? 'csv' : 'json';
      link.download = `logs_${startStr}_${endStr}.${extension}`;

      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);
    } catch (error) {
      showError(error.message || t('文件下载失败'));
    }
  }

  async function onDeleteExportTask(taskId) {
    try {
      const res = await API.delete(`/api/log/export/${taskId}`);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('任务已删除'));
        await fetchExportTasks();
      } else {
        throw new Error(message || t('删除失败'));
      }
    } catch (error) {
      showError(error.message || t('删除失败'));
    }
  }

  function formatFileSize(bytes) {
    if (!bytes && bytes !== 0) return '-';
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(2)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
  }

  const taskColumns = [
    {
      title: t('创建时间'),
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
      render: (createdAt) => (
        <Text>{createdAt ? timestamp2string(createdAt) : '-'}</Text>
      ),
    },
    {
      title: t('时间范围'),
      key: 'time_range',
      width: 280,
      render: (text, record) => (
        <Text>
          {record.start_time ? timestamp2string(record.start_time) : '-'} ~ {record.end_time ? timestamp2string(record.end_time) : '-'}
        </Text>
      ),
    },
    {
      title: t('格式'),
      dataIndex: 'format',
      key: 'format',
      width: 80,
      render: (format) => <Tag>{format?.toUpperCase?.() || format}</Tag>,
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      key: 'status',
      width: 120,
      render: (status, record) => {
        const config = statusMap[status] || { label: status, color: 'grey' };
        const tag = (
          <Tag color={config.color}>
            {status === 'processing' && (
              <Loader size={12} style={{ marginRight: 4, animation: 'spin 1s linear infinite' }} />
            )}
            {config.label}
          </Tag>
        );
        if (status === 'failed' && record.error_msg) {
          return (
            <Tooltip content={record.error_msg} showArrow position='topLeft'>
              {tag}
            </Tooltip>
          );
        }
        return tag;
      },
    },
    {
      title: t('文件大小'),
      dataIndex: 'file_size',
      key: 'file_size',
      width: 120,
      render: (fileSize) => <Text>{formatFileSize(fileSize)}</Text>,
    },
    {
      title: t('操作'),
      key: 'action',
      width: 150,
      render: (text, record) => (
        <Space>
          {record.status === 'success' && (
            <Button
              icon={<Download size={14} />}
              theme='light'
              type='tertiary'
              size='small'
              onClick={() => onDownloadExportTask(record)}
            >
              {t('下载')}
            </Button>
          )}
          <Popconfirm
            title={t('确认删除')}
            content={t('确定要删除此导出任务吗？')}
            onConfirm={() => onDeleteExportTask(record.id)}
            position='top'
          >
            <Button
              icon={<Trash2 size={14} />}
              type='danger'
              theme='light'
              size='small'
            >
              {t('删除')}
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  async function onCleanHistoryLog() {
    const { startTimestamp, endTimestamp, cleanMode } = cleanInputs;

    // 验证时间选择
    if (!startTimestamp || !endTimestamp) {
      return showError(t('请选择开始时间和结束时间'));
    }

    const startTs = Date.parse(startTimestamp);
    const endTs = Date.parse(endTimestamp);

    if (startTs > endTs) {
      return showError(t('开始时间不能晚于结束时间'));
    }

    // 根据清理模式显示不同的确认弹窗
    if (cleanMode === 'all') {
      Modal.confirm({
        title: t('确认删除日志'),
        content: (
          <Space vertical align='start'>
            <Text type='danger' strong>
              {t('警告：此操作将永久删除选定时间范围内的日志记录，不可恢复！')}
            </Text>
            <Text type='secondary'>
              {t('时间范围：')}
              {dayjs(startTimestamp).format('YYYY-MM-DD HH:mm:ss')} {t('至')} {dayjs(endTimestamp).format('YYYY-MM-DD HH:mm:ss')}
            </Text>
          </Space>
        ),
        okText: t('确认删除'),
        okType: 'danger',
        cancelText: t('取消'),
        onOk: () => executeClean(startTs, endTs, cleanMode),
      });
    } else {
      Modal.confirm({
        title: t('确认清理日志内容'),
        content: (
          <Space vertical align='start'>
            <Text>
              {t('将保留日志元数据，仅删除用户输入和模型响应内容')}
            </Text>
            <Text type='secondary'>
              {t('时间范围：')}
              {dayjs(startTimestamp).format('YYYY-MM-DD HH:mm:ss')} {t('至')} {dayjs(endTimestamp).format('YYYY-MM-DD HH:mm:ss')}
            </Text>
          </Space>
        ),
        okText: t('确认清理'),
        cancelText: t('取消'),
        onOk: () => executeClean(startTs, endTs, cleanMode),
      });
    }
  }

  async function executeClean(startTs, endTs, cleanMode) {
    try {
      setLoadingCleanHistoryLog(true);
      const res = await API.delete(
        `/api/log/?start_timestamp=${Math.floor(startTs / 1000)}&end_timestamp=${Math.floor(endTs / 1000)}&clean_mode=${cleanMode}`
      );
      const { success, message, data } = res.data;
      if (success) {
        showSuccess(`${data} ${t('条日志已清理！')}`);
        return;
      } else {
        throw new Error(t('日志清理失败：') + message);
      }
    } catch (error) {
      showError(error.message);
    } finally {
      setLoadingCleanHistoryLog(false);
    }
  }

  useEffect(() => {
    const currentInputs = {};
    for (let key in props.options) {
      if (Object.keys(inputs).includes(key)) {
        currentInputs[key] = props.options[key];
      }
    }
    setInputs(Object.assign(inputs, currentInputs));
    setInputsRow(structuredClone(currentInputs));
    refForm.current.setValues(currentInputs);
  }, [props.options]);

  return (
    <>
      <Spin spinning={loading}>
        <Form
          values={inputs}
          getFormApi={(formAPI) => (refForm.current = formAPI)}
          style={{ marginBottom: 15 }}
        >
          <Form.Section text={t('日志设置')}>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Switch
                  field={'LogConsumeEnabled'}
                  label={t('启用额度消费日志记录')}
                  size='default'
                  checkedText='｜'
                  uncheckedText='〇'
                  onChange={(value) => {
                    setInputs({
                      ...inputs,
                      LogConsumeEnabled: value,
                    });
                  }}
                />
              </Col>
            </Row>

            <Row>
              <Button size='default' onClick={onSubmit}>
                {t('保存日志设置')}
              </Button>
            </Row>
          </Form.Section>
        </Form>
      </Spin>

      {/* 历史日志导出区域 */}
      <Spin spinning={loadingExportLog}>
        <Form
          getFormApi={(formAPI) => (exportFormRef.current = formAPI)}
          style={{ marginBottom: 15 }}
        >
          <Form.Section text={t('历史日志导出')}>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.DatePicker
                  label={t('开始时间')}
                  field={'exportStartTimestamp'}
                  type='dateTime'
                  inputReadOnly={true}
                  initValue={exportInputs.startTimestamp}
                  onChange={(value) => {
                    setExportInputs({
                      ...exportInputs,
                      startTimestamp: value,
                    });
                  }}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.DatePicker
                  label={t('结束时间')}
                  field={'exportEndTimestamp'}
                  type='dateTime'
                  inputReadOnly={true}
                  initValue={exportInputs.endTimestamp}
                  onChange={(value) => {
                    setExportInputs({
                      ...exportInputs,
                      endTimestamp: value,
                    });
                  }}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.RadioGroup
                  label={t('导出格式')}
                  field={'exportFormat'}
                  type='button'
                  initValue={exportInputs.format}
                  onChange={(e) => {
                    setExportInputs({
                      ...exportInputs,
                      format: e.target.value,
                    });
                  }}
                >
                  <Radio value='json'>JSON</Radio>
                  <Radio value='csv'>CSV</Radio>
                </Form.RadioGroup>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Input
                  label={t('用户名筛选')}
                  field={'exportUsername'}
                  placeholder={t('可选，输入用户名')}
                  onChange={(value) => {
                    setExportInputs({
                      ...exportInputs,
                      username: value,
                    });
                  }}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Input
                  label={t('模型名筛选')}
                  field={'exportModelName'}
                  placeholder={t('可选，输入模型名')}
                  onChange={(value) => {
                    setExportInputs({
                      ...exportInputs,
                      modelName: value,
                    });
                  }}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.RadioGroup
                  label={t('输入输出内容')}
                  field={'exportBodyExportMode'}
                  type='button'
                  initValue={exportInputs.bodyExportMode}
                  onChange={(e) => {
                    setExportInputs({
                      ...exportInputs,
                      bodyExportMode: e.target.value,
                    });
                  }}
                >
                  <Radio value='full'>{t('完整')}</Radio>
                  <Radio value='truncated'>{t('截断')}</Radio>
                  <Radio value='none'>{t('不包含')}</Radio>
                </Form.RadioGroup>
                {exportInputs.bodyExportMode === 'truncated' && (
                  <Form.InputNumber
                    label={t('截断长度')}
                    field={'exportBodyExportLength'}
                    initValue={exportInputs.bodyExportLength}
                    min={1}
                    max={100000}
                    onChange={(value) => {
                      setExportInputs({
                        ...exportInputs,
                        bodyExportLength: value,
                      });
                    }}
                  />
                )}
              </Col>
            </Row>
            <Row>
              <Button
                type='primary'
                size='default'
                onClick={onExportLog}
                loading={loadingExportLog}
              >
                {t('导出日志')}
              </Button>
            </Row>
          </Form.Section>
        </Form>
      </Spin>

      {/* 导出任务列表 */}
      <Form.Section text={t('导出任务列表')}>
        <Table
          columns={taskColumns}
          dataSource={exportTasks}
          rowKey='id'
          loading={loadingTasks}
          pagination={false}
          size='middle'
          empty={
            <div style={{ padding: 30, textAlign: 'center' }}>
              <Text type='secondary'>{t('暂无导出任务')}</Text>
            </div>
          }
        />
      </Form.Section>

      {/* 历史日志清理区域 */}
      <Spin spinning={loadingCleanHistoryLog}>
        <Form
          getFormApi={(formAPI) => (cleanFormRef.current = formAPI)}
          style={{ marginBottom: 15 }}
        >
          <Form.Section text={t('历史日志清理')}>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.DatePicker
                  label={t('开始时间')}
                  field={'cleanStartTimestamp'}
                  type='dateTime'
                  inputReadOnly={true}
                  initValue={cleanInputs.startTimestamp}
                  onChange={(value) => {
                    setCleanInputs({
                      ...cleanInputs,
                      startTimestamp: value,
                    });
                  }}
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.DatePicker
                  label={t('结束时间')}
                  field={'cleanEndTimestamp'}
                  type='dateTime'
                  inputReadOnly={true}
                  initValue={cleanInputs.endTimestamp}
                  onChange={(value) => {
                    setCleanInputs({
                      ...cleanInputs,
                      endTimestamp: value,
                    });
                  }}
                />
              </Col>
            </Row>

            <Row gutter={16}>
              <Col xs={24} sm={24} md={16} lg={16} xl={16}>
                <Form.RadioGroup
                  field={'cleanMode'}
                  label={t('清理模式')}
                  initValue={cleanInputs.cleanMode}
                  onChange={(e) => {
                    setCleanInputs({
                      ...cleanInputs,
                      cleanMode: e.target.value,
                    });
                  }}
                >
                  <Form.Radio value='all'>
                    <Space vertical align='start'>
                      <Text strong>{t('全部删除')}</Text>
                      <Text type='secondary' size='small'>
                        {t('删除整行日志记录，不可恢复')}
                      </Text>
                    </Space>
                  </Form.Radio>
                  <Form.Radio value='body_only'>
                    <Space vertical align='start'>
                      <Text strong>{t('仅清理内容')}</Text>
                      <Text type='secondary' size='small'>
                        {t('保留日志元数据，仅删除 user_input 和 response_body')}
                      </Text>
                    </Space>
                  </Form.Radio>
                </Form.RadioGroup>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col xs={24} sm={24} md={16} lg={16} xl={16}>
                <Button
                  type='danger'
                  size='default'
                  onClick={onCleanHistoryLog}
                  loading={loadingCleanHistoryLog}
                >
                  {t('清除历史日志')}
                </Button>
              </Col>
            </Row>
          </Form.Section>
        </Form>
      </Spin>
    </>
  );
}
