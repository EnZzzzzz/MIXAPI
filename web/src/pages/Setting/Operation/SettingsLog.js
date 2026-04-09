import React, { useEffect, useState, useRef } from 'react';
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
} from '@douyinfe/semi-ui';
import dayjs from 'dayjs';
import { useTranslation } from 'react-i18next';
import {
  compareObjects,
  API,
  showError,
  showSuccess,
  showWarning,
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

      const params = new URLSearchParams({
        start_timestamp: startTimestamp,
        end_timestamp: endTimestamp,
        format: exportInputs.format,
      });

      if (exportInputs.username) {
        params.append('username', exportInputs.username);
      }
      if (exportInputs.modelName) {
        params.append('model_name', exportInputs.modelName);
      }

      const response = await API.get(`/api/log/export?${params.toString()}`, {
        responseType: 'blob',
      });

      // 创建下载链接
      const blob = new Blob([response.data]);
      const url = window.URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;

      // 生成文件名
      const startStr = dayjs(exportInputs.startTimestamp).format('YYYYMMDD');
      const endStr = dayjs(exportInputs.endTimestamp).format('YYYYMMDD');
      const extension = exportInputs.format === 'csv' ? 'csv' : 'json';
      link.download = `logs_${startStr}_${endStr}.${extension}`;

      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      window.URL.revokeObjectURL(url);

      showSuccess(t('日志导出成功'));
    } catch (error) {
      showError(error.message || t('日志导出失败'));
    } finally {
      setLoadingExportLog(false);
    }
  }

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
