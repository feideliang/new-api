/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useState, useEffect } from 'react';
import { Card, Table, Select, Button, Typography, Space, Toast } from '@douyinfe/semi-ui';
import { API } from '../../helpers';
import { useTranslation } from 'react-i18next';

const { Text, Title } = Typography;
const { Option } = Select;

const TIME_RANGES = [
  { value: '7d', label: '最近7天', seconds: 7 * 24 * 3600 },
  { value: '30d', label: '最近30天', seconds: 30 * 24 * 3600 },
  { value: '90d', label: '最近90天', seconds: 90 * 24 * 3600 },
  { value: 'all', label: '全部时间', seconds: 0 },
];

const ToolStats = () => {
  const { t } = useTranslation();
  const [stats, setStats] = useState([]);
  const [loading, setLoading] = useState(false);
  const [timeRange, setTimeRange] = useState('7d');

  const fetchStats = async (range) => {
    setLoading(true);
    try {
      const now = Math.floor(Date.now() / 1000);
      const rangeConfig = TIME_RANGES.find((r) => r.value === range);
      const startTimestamp = rangeConfig?.seconds ? now - rangeConfig.seconds : 0;

      const res = await API.get(`/api/log/tool_stat?start_timestamp=${startTimestamp}&end_timestamp=${now}`);

      if (res.data?.success && Array.isArray(res.data.data)) {
        setStats(res.data.data);
      } else {
        setStats([]);
        Toast.warning(res.data?.message || '获取数据失败');
      }
    } catch (err) {
      console.error('Failed to fetch tool stats:', err);
      setStats([]);
      Toast.error('数据加载失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchStats(timeRange);
  }, [timeRange]);

  const totalCount = stats.reduce((sum, s) => sum + s.call_count, 0);
  const uniqueTools = stats.length;
  const topTool = stats[0];
  const topToolProportion = topTool && totalCount > 0
    ? ((topTool.call_count / totalCount) * 100).toFixed(1)
    : '0';

  const columns = [
    {
      title: '#',
      dataIndex: 'index',
      key: 'index',
      width: 80,
      render: (text, record, index) => (
        <Text strong>{index + 1}</Text>
      ),
    },
    {
      title: t('工具 / 技能名称'),
      dataIndex: 'tool_name',
      key: 'tool_name',
      render: (text) => (
        <Text strong copyable>{text}</Text>
      ),
    },
    {
      title: t('调用次数'),
      dataIndex: 'call_count',
      key: 'call_count',
      width: 150,
      align: 'right',
      render: (text) => (
        <Text type="tertiary">{text?.toLocaleString()}</Text>
      ),
    },
    {
      title: t('占比'),
      dataIndex: 'proportion',
      key: 'proportion',
      width: 120,
      align: 'right',
      render: (text, record) => {
        const proportion = totalCount > 0 ? (record.call_count / totalCount) * 100 : 0;
        return <Text type="tertiary">{proportion.toFixed(1)}%</Text>;
      },
    },
  ];

  return (
    <div className="mt-[60px] px-6">
      <Space vertical align="start" style={{ width: '100%' }} spacing={24}>
        {/* Header */}
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', width: '100%' }}>
          <Title heading={4} style={{ margin: 0 }}>{t('技能调用统计')}</Title>
          <Space>
            <Select value={timeRange} onChange={(v) => setTimeRange(v)} style={{ width: 130 }}>
              {TIME_RANGES.map((r) => (
                <Option key={r.value} value={r.value}>{r.label}</Option>
              ))}
            </Select>
            <Button theme="light" onClick={() => fetchStats(timeRange)} loading={loading}>
              {t('刷新')}
            </Button>
          </Space>
        </div>

        {/* Summary Cards */}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '16px', width: '100%' }}>
          <Card>
            <Text type="tertiary" size="small">{t('总调用次数')}</Text>
            <Title heading={3} style={{ marginTop: 8, marginBottom: 0 }}>
              {loading ? '...' : totalCount.toLocaleString()}
            </Title>
          </Card>
          <Card>
            <Text type="tertiary" size="small">{t('工具数量')}</Text>
            <Title heading={3} style={{ marginTop: 8, marginBottom: 0 }}>
              {loading ? '...' : uniqueTools.toLocaleString()}
            </Title>
          </Card>
          <Card>
            <Text type="tertiary" size="small">{t('最常用工具')}</Text>
            <Title heading={3} style={{ marginTop: 8, marginBottom: 0 }}>
              {loading || !topTool ? '—' : topTool.tool_name}
            </Title>
          </Card>
          <Card>
            <Text type="tertiary" size="small">{t('最高占比')}</Text>
            <Title heading={3} style={{ marginTop: 8, marginBottom: 0 }}>
              {loading ? '...' : `${topToolProportion}%`}
            </Title>
          </Card>
        </div>

        {/* Ranking Table */}
        <Card
          title={t('调用次数排名')}
          headerExtraContent={
            totalCount > 0 && (
              <Text type="tertiary" size="small">
                {t('总计')}: {totalCount.toLocaleString()} {t('次调用')}
              </Text>
            )
          }
        >
          <Table
            columns={columns}
            dataSource={stats}
            rowKey={(record) => record.tool_name}
            loading={loading}
            pagination={false}
            empty={
              <div style={{ textAlign: 'center', padding: '40px 0' }}>
                <Text type="tertiary">
                  {t('未找到工具使用数据。请启用 LOG_REQUEST_TOOLS=true 并发送带有 tools 的请求。')}
                </Text>
              </div>
            }
          />
        </Card>
      </Space>
    </div>
  );
};

export default ToolStats;