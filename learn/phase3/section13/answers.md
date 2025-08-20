# Section 13 Quiz Answers

## 📋 Answer Key

Here are the correct answers and explanations for the Section 13 quiz.

---

## **Multiple Choice Questions**

### **Question 1: Dashboard Information Hierarchy**
**Answer: B) Information hierarchy with primary metrics prominently displayed**

**Explanation**: The most important principle in dashboard design is information hierarchy, ensuring that the most critical metrics are displayed prominently while secondary information is organized logically and easily accessible.

### **Question 2: Real-time Data Updates**
**Answer: D) Both B and C**

**Explanation**: Both WebSocket connections and Server-Sent Events are excellent for real-time dashboard updates. WebSockets provide bidirectional communication, while SSE is perfect for one-way server-to-client updates.

### **Question 3: Dashboard Types**
**Answer: B) Executive Dashboard**

**Explanation**: Executive dashboards focus on high-level overview and KPIs, providing strategic insights for decision-makers rather than detailed technical information.

### **Question 4: Data Visualization**
**Answer: B) Line chart**

**Explanation**: Line charts are best for showing trends over time as they clearly display how values change across a continuous time period, making it easy to identify patterns and trends.

### **Question 5: Performance Monitoring**
**Answer: B) To show system health at a glance**

**Explanation**: Performance gauges are designed to provide immediate visual feedback about system health, allowing users to quickly assess the status of critical metrics.

### **Question 6: Dashboard Responsiveness**
**Answer: D) All of the above**

**Explanation**: Responsive design is important for dashboards to work on mobile devices, adapt to different screen sizes, and improve overall user experience across all platforms.

### **Question 7: Color Schemes**
**Answer: B) Data type and meaning**

**Explanation**: Colors in dashboard design should be chosen based on data type and meaning, using consistent color schemes that help users quickly understand and interpret the information.

### **Question 8: Interactive Elements**
**Answer: B) They allow users to explore data and drill down**

**Explanation**: Interactive elements enable users to explore data more deeply, drill down into specific details, and customize their view of the information, making dashboards more useful and engaging.

---

## **True/False Questions**

### **Question 9**
**Answer: False**

**Explanation**: Dashboards should display the most relevant and actionable information, not all available data. Too much information can overwhelm users and reduce the dashboard's effectiveness.

### **Question 10**
**Answer: False**

**Explanation**: Real-time updates should update specific components without refreshing the entire page, providing a smooth user experience and maintaining the user's current context.

### **Question 11**
**Answer: True**

**Explanation**: Performance metrics are crucial for technical dashboards as they help monitor system health, identify issues, and ensure optimal operation of the blockchain network.

### **Question 12**
**Answer: True**

**Explanation**: User dashboards should focus on personal information like wallet balances, transaction history, and account statistics that are relevant to individual users.

### **Question 13**
**Answer: False**

**Explanation**: Dashboard accessibility is important for all applications, not just government ones. It ensures that users with disabilities can effectively use the dashboard and access important information.

### **Question 14**
**Answer: False**

**Explanation**: Data visualization should prioritize clarity over aesthetics. While visual appeal is important, the primary goal is to communicate information clearly and effectively.

---

## **Practical Questions**

### **Question 15: Dashboard Layout**

```jsx
import React, { useState, useEffect } from 'react';
import { LineChart, BarChart, PieChart } from 'recharts';

const DashboardLayout = () => {
  const [metrics, setMetrics] = useState({});
  const [chartData, setChartData] = useState({});

  useEffect(() => {
    fetchDashboardData();
  }, []);

  const fetchDashboardData = async () => {
    try {
      const [metricsRes, chartsRes] = await Promise.all([
        fetch('/api/v1/dashboard/metrics'),
        fetch('/api/v1/dashboard/charts')
      ]);
      
      const [metricsData, chartsData] = await Promise.all([
        metricsRes.json(),
        chartsRes.json()
      ]);
      
      setMetrics(metricsData);
      setChartData(chartsData);
    } catch (error) {
      console.error('Failed to fetch dashboard data:', error);
    }
  };

  return (
    <div className="dashboard-layout">
      <header className="dashboard-header">
        <h1>Blockchain Network Dashboard</h1>
        <div className="dashboard-controls">
          <select className="time-range-selector">
            <option value="1h">Last Hour</option>
            <option value="24h">Last 24 Hours</option>
            <option value="7d">Last 7 Days</option>
            <option value="30d">Last 30 Days</option>
          </select>
          <button onClick={fetchDashboardData}>Refresh</button>
        </div>
      </header>

      {/* Metrics Cards */}
      <section className="metrics-section">
        <div className="metrics-grid">
          <MetricCard
            title="Total Transactions"
            value={metrics.totalTransactions || 0}
            change={metrics.transactionChange || 0}
            icon="📊"
            color="blue"
          />
          <MetricCard
            title="Active Wallets"
            value={metrics.activeWallets || 0}
            change={metrics.walletChange || 0}
            icon="👛"
            color="green"
          />
          <MetricCard
            title="Network Hash Rate"
            value={formatHashRate(metrics.hashRate || 0)}
            change={metrics.hashRateChange || 0}
            icon="⛏️"
            color="orange"
          />
          <MetricCard
            title="Block Height"
            value={metrics.blockHeight || 0}
            change={metrics.blockChange || 0}
            icon="🔗"
            color="purple"
          />
        </div>
      </section>

      {/* Charts Section */}
      <section className="charts-section">
        <div className="charts-grid">
          <ChartWidget
            title="Transaction Volume Over Time"
            type="line"
            data={chartData.transactionVolume || []}
            height={300}
          />
          <ChartWidget
            title="Network Activity Distribution"
            type="bar"
            data={chartData.networkActivity || []}
            height={300}
          />
          <ChartWidget
            title="Wallet Type Distribution"
            type="pie"
            data={chartData.walletDistribution || []}
            height={300}
          />
        </div>
      </section>
    </div>
  );
};

const MetricCard = ({ title, value, change, icon, color }) => (
  <div className={`metric-card metric-card--${color}`}>
    <div className="metric-card__header">
      <span className="metric-card__icon">{icon}</span>
      <h3 className="metric-card__title">{title}</h3>
    </div>
    <div className="metric-card__value">{value}</div>
    <div className={`metric-card__change metric-card__change--${change >= 0 ? 'positive' : 'negative'}`}>
      {change >= 0 ? '+' : ''}{change}%
    </div>
  </div>
);

const ChartWidget = ({ title, type, data, height }) => (
  <div className="chart-widget">
    <h3 className="chart-widget__title">{title}</h3>
    <div className="chart-widget__content">
      {type === 'line' && (
        <LineChart width={400} height={height} data={data}>
          {/* Chart configuration */}
        </LineChart>
      )}
      {type === 'bar' && (
        <BarChart width={400} height={height} data={data}>
          {/* Chart configuration */}
        </BarChart>
      )}
      {type === 'pie' && (
        <PieChart width={400} height={height} data={data}>
          {/* Chart configuration */}
        </PieChart>
      )}
    </div>
  </div>
);

const styles = `
.dashboard-layout {
  padding: 24px;
  background: #f8f9fa;
  min-height: 100vh;
}

.dashboard-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 32px;
  padding: 20px;
  background: white;
  border-radius: 12px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 20px;
  margin-bottom: 32px;
}

.charts-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(400px, 1fr));
  gap: 20px;
}

.metric-card {
  background: white;
  border-radius: 12px;
  padding: 20px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
  transition: transform 0.2s ease;
}

.metric-card:hover {
  transform: translateY(-2px);
}

.chart-widget {
  background: white;
  border-radius: 12px;
  padding: 20px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

/* Responsive design */
@media (max-width: 768px) {
  .dashboard-header {
    flex-direction: column;
    gap: 16px;
  }
  
  .metrics-grid {
    grid-template-columns: 1fr;
  }
  
  .charts-grid {
    grid-template-columns: 1fr;
  }
}
`;

export default DashboardLayout;
```

### **Question 16: Real-time Data Integration**

```jsx
import React, { useState, useEffect, useRef } from 'react';

const RealTimeDashboard = () => {
  const [metrics, setMetrics] = useState({});
  const [isConnected, setIsConnected] = useState(false);
  const wsRef = useRef(null);

  useEffect(() => {
    setupWebSocket();
    return () => {
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, []);

  const setupWebSocket = () => {
    const ws = new WebSocket('ws://localhost:8080/dashboard/stream');
    wsRef.current = ws;

    ws.onopen = () => {
      setIsConnected(true);
      console.log('WebSocket connected');
    };

    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      handleRealTimeUpdate(data);
    };

    ws.onclose = () => {
      setIsConnected(false);
      console.log('WebSocket disconnected');
      // Attempt to reconnect after 3 seconds
      setTimeout(setupWebSocket, 3000);
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
  };

  const handleRealTimeUpdate = (data) => {
    switch (data.type) {
      case 'metrics_update':
        setMetrics(prev => ({ ...prev, ...data.metrics }));
        break;
      case 'chart_update':
        // Update chart data
        break;
      case 'alert':
        showAlert(data.message);
        break;
      default:
        console.log('Unknown update type:', data.type);
    }
  };

  const showAlert = (message) => {
    // Display alert notification
    console.log('Alert:', message);
  };

  return (
    <div className="real-time-dashboard">
      <div className="connection-status">
        <div className={`status-indicator ${isConnected ? 'connected' : 'disconnected'}`}>
          {isConnected ? '🟢 Connected' : '🔴 Disconnected'}
        </div>
      </div>

      <div className="real-time-metrics">
        <h2>Real-time Metrics</h2>
        <div className="metrics-grid">
          <RealTimeMetricCard
            title="Transactions/sec"
            value={metrics.transactionsPerSecond || 0}
            trend={metrics.transactionTrend || 'stable'}
          />
          <RealTimeMetricCard
            title="Active Connections"
            value={metrics.activeConnections || 0}
            trend={metrics.connectionTrend || 'stable'}
          />
          <RealTimeMetricCard
            title="Block Time"
            value={metrics.blockTime || 0}
            trend={metrics.blockTimeTrend || 'stable'}
          />
          <RealTimeMetricCard
            title="Network Load"
            value={metrics.networkLoad || 0}
            trend={metrics.networkLoadTrend || 'stable'}
          />
        </div>
      </div>

      <div className="live-charts">
        <h2>Live Charts</h2>
        <div className="charts-grid">
          <LiveChart
            title="Transaction Volume"
            data={metrics.transactionVolume || []}
            type="line"
          />
          <LiveChart
            title="Network Activity"
            data={metrics.networkActivity || []}
            type="bar"
          />
        </div>
      </div>
    </div>
  );
};

const RealTimeMetricCard = ({ title, value, trend }) => (
  <div className="real-time-metric-card">
    <h3>{title}</h3>
    <div className="metric-value">{value}</div>
    <div className={`metric-trend metric-trend--${trend}`}>
      {trend === 'up' && '↗️'}
      {trend === 'down' && '↘️'}
      {trend === 'stable' && '→'}
    </div>
  </div>
);

const LiveChart = ({ title, data, type }) => (
  <div className="live-chart">
    <h3>{title}</h3>
    <div className="chart-container">
      {/* Chart implementation */}
      <div className="chart-placeholder">
        {type} chart with {data.length} data points
      </div>
    </div>
  </div>
);

// Server-Sent Events alternative
const useSSEData = (url) => {
  const [data, setData] = useState(null);

  useEffect(() => {
    const eventSource = new EventSource(url);

    eventSource.onmessage = (event) => {
      const newData = JSON.parse(event.data);
      setData(newData);
    };

    eventSource.onerror = (error) => {
      console.error('SSE error:', error);
      eventSource.close();
    };

    return () => eventSource.close();
  }, [url]);

  return data;
};
```

### **Question 17: Interactive Charts**

```jsx
import React, { useState } from 'react';
import { LineChart, BarChart, PieChart, Cell } from 'recharts';

const InteractiveCharts = () => {
  const [selectedData, setSelectedData] = useState(null);
  const [filters, setFilters] = useState({});
  const [drillDownLevel, setDrillDownLevel] = useState(0);

  const handleChartClick = (data) => {
    setSelectedData(data);
    if (drillDownLevel < 2) {
      setDrillDownLevel(prev => prev + 1);
    }
  };

  const handleFilterChange = (filterType, value) => {
    setFilters(prev => ({ ...prev, [filterType]: value }));
  };

  const filteredData = applyFilters(chartData, filters);

  return (
    <div className="interactive-charts">
      <div className="chart-controls">
        <div className="filters">
          <select
            value={filters.timeRange || 'all'}
            onChange={(e) => handleFilterChange('timeRange', e.target.value)}
          >
            <option value="all">All Time</option>
            <option value="1h">Last Hour</option>
            <option value="24h">Last 24 Hours</option>
            <option value="7d">Last 7 Days</option>
          </select>
          
          <select
            value={filters.transactionType || 'all'}
            onChange={(e) => handleFilterChange('transactionType', e.target.value)}
          >
            <option value="all">All Types</option>
            <option value="transfer">Transfers</option>
            <option value="contract">Smart Contracts</option>
            <option value="mining">Mining Rewards</option>
          </select>
        </div>

        <div className="drill-down-controls">
          <button
            onClick={() => setDrillDownLevel(prev => Math.max(0, prev - 1))}
            disabled={drillDownLevel === 0}
          >
            ← Back
          </button>
          <span>Level: {drillDownLevel}</span>
        </div>
      </div>

      <div className="charts-container">
        <InteractiveLineChart
          data={filteredData}
          onPointClick={handleChartClick}
          drillDownLevel={drillDownLevel}
        />
        
        <InteractiveBarChart
          data={filteredData}
          onBarClick={handleChartClick}
          drillDownLevel={drillDownLevel}
        />
        
        <InteractivePieChart
          data={filteredData}
          onSliceClick={handleChartClick}
          drillDownLevel={drillDownLevel}
        />
      </div>

      {selectedData && (
        <div className="data-details">
          <h3>Selected Data Details</h3>
          <pre>{JSON.stringify(selectedData, null, 2)}</pre>
        </div>
      )}
    </div>
  );
};

const InteractiveLineChart = ({ data, onPointClick, drillDownLevel }) => (
  <div className="chart-widget">
    <h3>Transaction Trends (Level {drillDownLevel})</h3>
    <LineChart width={500} height={300} data={data}>
      {/* Chart configuration with click handlers */}
      {data.map((entry, index) => (
        <g key={index}>
          <circle
            cx={getXPosition(entry, index)}
            cy={getYPosition(entry)}
            r={4}
            fill="#8884d8"
            onClick={() => onPointClick(entry)}
            style={{ cursor: 'pointer' }}
          />
        </g>
      ))}
    </LineChart>
  </div>
);

const InteractiveBarChart = ({ data, onBarClick, drillDownLevel }) => (
  <div className="chart-widget">
    <h3>Transaction Volume (Level {drillDownLevel})</h3>
    <BarChart width={500} height={300} data={data}>
      {/* Chart configuration with click handlers */}
      {data.map((entry, index) => (
        <rect
          key={index}
          x={getBarXPosition(entry, index)}
          y={getBarYPosition(entry)}
          width={getBarWidth()}
          height={getBarHeight(entry)}
          fill="#8884d8"
          onClick={() => onBarClick(entry)}
          style={{ cursor: 'pointer' }}
        />
      ))}
    </BarChart>
  </div>
);

const InteractivePieChart = ({ data, onSliceClick, drillDownLevel }) => (
  <div className="chart-widget">
    <h3>Transaction Distribution (Level {drillDownLevel})</h3>
    <PieChart width={400} height={300}>
      {data.map((entry, index) => (
        <Cell
          key={`cell-${index}`}
          fill={getPieColor(index)}
          onClick={() => onSliceClick(entry)}
          style={{ cursor: 'pointer' }}
        />
      ))}
    </PieChart>
  </div>
);

// Utility functions for chart positioning
const getXPosition = (entry, index) => {
  // Calculate X position based on data and chart dimensions
  return index * 50 + 50;
};

const getYPosition = (entry) => {
  // Calculate Y position based on value and chart dimensions
  return 300 - (entry.value * 2);
};

const getBarXPosition = (entry, index) => {
  return index * 60 + 30;
};

const getBarYPosition = (entry) => {
  return 300 - (entry.value * 2);
};

const getBarWidth = () => 40;

const getBarHeight = (entry) => {
  return entry.value * 2;
};

const getPieColor = (index) => {
  const colors = ['#0088FE', '#00C49F', '#FFBB28', '#FF8042', '#8884D8'];
  return colors[index % colors.length];
};

const applyFilters = (data, filters) => {
  let filtered = [...data];
  
  if (filters.timeRange && filters.timeRange !== 'all') {
    const now = new Date();
    const timeRanges = {
      '1h': now.getTime() - (60 * 60 * 1000),
      '24h': now.getTime() - (24 * 60 * 60 * 1000),
      '7d': now.getTime() - (7 * 24 * 60 * 60 * 1000),
    };
    
    filtered = filtered.filter(item => 
      new Date(item.timestamp).getTime() > timeRanges[filters.timeRange]
    );
  }
  
  if (filters.transactionType && filters.transactionType !== 'all') {
    filtered = filtered.filter(item => 
      item.type === filters.transactionType
    );
  }
  
  return filtered;
};
```

### **Question 18: Performance Monitoring**

```jsx
import React, { useState, useEffect } from 'react';

const PerformanceMonitoring = () => {
  const [performanceData, setPerformanceData] = useState({});
  const [alerts, setAlerts] = useState([]);
  const [thresholds, setThresholds] = useState({
    cpu: 80,
    memory: 85,
    disk: 90,
    network: 75
  });

  useEffect(() => {
    const interval = setInterval(fetchPerformanceData, 5000);
    return () => clearInterval(interval);
  }, []);

  const fetchPerformanceData = async () => {
    try {
      const response = await fetch('/api/v1/performance/metrics');
      const data = await response.json();
      setPerformanceData(data);
      checkAlerts(data);
    } catch (error) {
      console.error('Failed to fetch performance data:', error);
    }
  };

  const checkAlerts = (data) => {
    const newAlerts = [];
    
    if (data.cpu > thresholds.cpu) {
      newAlerts.push({
        type: 'warning',
        message: `CPU usage is high: ${data.cpu}%`,
        timestamp: new Date().toISOString()
      });
    }
    
    if (data.memory > thresholds.memory) {
      newAlerts.push({
        type: 'critical',
        message: `Memory usage is critical: ${data.memory}%`,
        timestamp: new Date().toISOString()
      });
    }
    
    if (data.disk > thresholds.disk) {
      newAlerts.push({
        type: 'critical',
        message: `Disk usage is critical: ${data.disk}%`,
        timestamp: new Date().toISOString()
      });
    }
    
    if (newAlerts.length > 0) {
      setAlerts(prev => [...newAlerts, ...prev.slice(0, 9)]);
    }
  };

  return (
    <div className="performance-monitoring">
      <div className="monitoring-header">
        <h2>System Performance Monitoring</h2>
        <div className="threshold-controls">
          <button onClick={() => setThresholds(prev => ({ ...prev, cpu: prev.cpu + 5 }))}>
            Increase CPU Threshold
          </button>
          <button onClick={() => setThresholds(prev => ({ ...prev, memory: prev.memory + 5 }))}>
            Increase Memory Threshold
          </button>
        </div>
      </div>

      <div className="performance-gauges">
        <PerformanceGauge
          title="CPU Usage"
          value={performanceData.cpu || 0}
          threshold={thresholds.cpu}
          color="#3b82f6"
          unit="%"
        />
        <PerformanceGauge
          title="Memory Usage"
          value={performanceData.memory || 0}
          threshold={thresholds.memory}
          color="#10b981"
          unit="%"
        />
        <PerformanceGauge
          title="Disk Usage"
          value={performanceData.disk || 0}
          threshold={thresholds.disk}
          color="#f59e0b"
          unit="%"
        />
        <PerformanceGauge
          title="Network I/O"
          value={performanceData.network || 0}
          threshold={thresholds.network}
          color="#ef4444"
          unit="%"
        />
      </div>

      <div className="alerts-section">
        <h3>System Alerts</h3>
        <div className="alerts-list">
          {alerts.map((alert, index) => (
            <AlertItem key={index} alert={alert} />
          ))}
        </div>
      </div>

      <div className="performance-charts">
        <PerformanceChart
          title="Performance Over Time"
          data={performanceData.history || []}
        />
      </div>
    </div>
  );
};

const PerformanceGauge = ({ title, value, threshold, color, unit }) => {
  const percentage = Math.min(value, 100);
  const isOverThreshold = value > threshold;
  
  return (
    <div className="performance-gauge">
      <h3>{title}</h3>
      <div className="gauge-container">
        <svg width="120" height="120" viewBox="0 0 120 120">
          {/* Background circle */}
          <circle
            cx="60"
            cy="60"
            r="50"
            fill="none"
            stroke="#e5e7eb"
            strokeWidth="10"
          />
          
          {/* Value circle */}
          <circle
            cx="60"
            cy="60"
            r="50"
            fill="none"
            stroke={isOverThreshold ? '#ef4444' : color}
            strokeWidth="10"
            strokeDasharray={`${(percentage / 100) * 314} 314`}
            transform="rotate(-90 60 60)"
            strokeLinecap="round"
          />
          
          {/* Threshold indicator */}
          <circle
            cx="60"
            cy="60"
            r="50"
            fill="none"
            stroke="#f59e0b"
            strokeWidth="2"
            strokeDasharray={`${(threshold / 100) * 314} 314`}
            transform="rotate(-90 60 60)"
            opacity="0.5"
          />
        </svg>
        
        <div className="gauge-value">
          <span className="value">{value}{unit}</span>
          <span className="threshold">Threshold: {threshold}{unit}</span>
        </div>
      </div>
      
      {isOverThreshold && (
        <div className="gauge-warning">
          ⚠️ Above threshold
        </div>
      )}
    </div>
  );
};

const AlertItem = ({ alert }) => (
  <div className={`alert-item alert-item--${alert.type}`}>
    <div className="alert-icon">
      {alert.type === 'critical' ? '🚨' : '⚠️'}
    </div>
    <div className="alert-content">
      <div className="alert-message">{alert.message}</div>
      <div className="alert-timestamp">
        {new Date(alert.timestamp).toLocaleTimeString()}
      </div>
    </div>
  </div>
);

const PerformanceChart = ({ title, data }) => (
  <div className="performance-chart">
    <h3>{title}</h3>
    <div className="chart-container">
      {/* Chart implementation */}
      <div className="chart-placeholder">
        Performance trend chart with {data.length} data points
      </div>
    </div>
  </div>
);

const styles = `
.performance-monitoring {
  padding: 24px;
  background: #f8f9fa;
}

.performance-gauges {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 20px;
  margin-bottom: 24px;
}

.performance-gauge {
  background: white;
  border-radius: 12px;
  padding: 20px;
  text-align: center;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.gauge-container {
  position: relative;
  display: inline-block;
}

.gauge-value {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  text-align: center;
}

.gauge-value .value {
  display: block;
  font-size: 18px;
  font-weight: bold;
}

.gauge-value .threshold {
  display: block;
  font-size: 12px;
  color: #666;
}

.gauge-warning {
  color: #ef4444;
  font-size: 14px;
  margin-top: 8px;
}

.alerts-section {
  background: white;
  border-radius: 12px;
  padding: 20px;
  margin-bottom: 24px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.alert-item {
  display: flex;
  align-items: center;
  padding: 12px;
  margin-bottom: 8px;
  border-radius: 8px;
  border-left: 4px solid;
}

.alert-item--critical {
  background: #fef2f2;
  border-left-color: #ef4444;
}

.alert-item--warning {
  background: #fffbeb;
  border-left-color: #f59e0b;
}

.alert-icon {
  margin-right: 12px;
  font-size: 20px;
}

.alert-message {
  font-weight: 600;
  margin-bottom: 4px;
}

.alert-timestamp {
  font-size: 12px;
  color: #666;
}
`;
```

---

## **Bonus Challenge: Complete Dashboard System**

```jsx
// Complete dashboard system
import React, { useState, useEffect } from 'react';
import { BrowserRouter as Router, Route, Routes } from 'react-router-dom';

const DashboardSystem = () => {
  return (
    <Router>
      <div className="dashboard-system">
        <Navigation />
        <Routes>
          <Route path="/executive" element={<ExecutiveDashboard />} />
          <Route path="/technical" element={<TechnicalDashboard />} />
          <Route path="/user" element={<UserDashboard />} />
          <Route path="/" element={<ExecutiveDashboard />} />
        </Routes>
      </div>
    </Router>
  );
};

const Navigation = () => (
  <nav className="dashboard-nav">
    <div className="nav-brand">Blockchain Dashboard</div>
    <div className="nav-links">
      <a href="/executive">Executive</a>
      <a href="/technical">Technical</a>
      <a href="/user">User</a>
    </div>
  </nav>
);

// Main dashboard component with all features
const CompleteDashboard = () => {
  const [dashboardData, setDashboardData] = useState({});
  const [realTimeUpdates, setRealTimeUpdates] = useState({});
  const [alerts, setAlerts] = useState([]);
  const [userPreferences, setUserPreferences] = useState({});

  useEffect(() => {
    // Initialize dashboard
    initializeDashboard();
    
    // Set up real-time connections
    setupRealTimeConnections();
    
    // Set up accessibility features
    setupAccessibility();
  }, []);

  const initializeDashboard = async () => {
    try {
      const [metrics, charts, alerts] = await Promise.all([
        fetch('/api/v1/dashboard/metrics').then(r => r.json()),
        fetch('/api/v1/dashboard/charts').then(r => r.json()),
        fetch('/api/v1/dashboard/alerts').then(r => r.json())
      ]);
      
      setDashboardData({ metrics, charts });
      setAlerts(alerts);
    } catch (error) {
      console.error('Failed to initialize dashboard:', error);
    }
  };

  const setupRealTimeConnections = () => {
    // WebSocket for real-time updates
    const ws = new WebSocket('ws://localhost:8080/dashboard/stream');
    
    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      handleRealTimeUpdate(data);
    };
    
    // Server-Sent Events for alerts
    const eventSource = new EventSource('/api/v1/dashboard/alerts/stream');
    
    eventSource.onmessage = (event) => {
      const alert = JSON.parse(event.data);
      setAlerts(prev => [alert, ...prev.slice(0, 9)]);
    };
  };

  const setupAccessibility = () => {
    // Keyboard navigation
    document.addEventListener('keydown', handleKeyboardNavigation);
    
    // Screen reader support
    setupScreenReaderSupport();
    
    // High contrast mode
    setupHighContrastMode();
  };

  const handleRealTimeUpdate = (data) => {
    setRealTimeUpdates(prev => ({ ...prev, ...data }));
  };

  const handleKeyboardNavigation = (event) => {
    // Implement keyboard navigation
  };

  const setupScreenReaderSupport = () => {
    // Add ARIA labels and descriptions
  };

  const setupHighContrastMode = () => {
    // Implement high contrast mode toggle
  };

  return (
    <div className="complete-dashboard">
      <header className="dashboard-header">
        <h1>Blockchain Network Dashboard</h1>
        <div className="dashboard-controls">
          <AccessibilityControls />
          <ThemeToggle />
          <RefreshButton />
        </div>
      </header>

      <main className="dashboard-content">
        <aside className="dashboard-sidebar">
          <QuickStats data={dashboardData.metrics} />
          <AlertPanel alerts={alerts} />
        </aside>

        <section className="dashboard-main">
          <MetricsGrid data={dashboardData.metrics} />
          <ChartsSection data={dashboardData.charts} />
          <PerformanceMonitoring />
        </section>
      </main>

      <footer className="dashboard-footer">
        <div className="connection-status">
          <span className="status-indicator connected">🟢 Live</span>
          <span>Last updated: {new Date().toLocaleTimeString()}</span>
        </div>
      </footer>
    </div>
  );
};

// Accessibility components
const AccessibilityControls = () => (
  <div className="accessibility-controls">
    <button onClick={() => toggleHighContrast()}>High Contrast</button>
    <button onClick={() => increaseFontSize()}>A+</button>
    <button onClick={() => decreaseFontSize()}>A-</button>
  </div>
);

const ThemeToggle = () => {
  const [isDark, setIsDark] = useState(false);
  
  const toggleTheme = () => {
    setIsDark(!isDark);
    document.body.classList.toggle('dark-theme');
  };
  
  return (
    <button onClick={toggleTheme}>
      {isDark ? '☀️' : '🌙'}
    </button>
  );
};

const RefreshButton = () => (
  <button onClick={() => window.location.reload()}>
    🔄 Refresh
  </button>
);

export default DashboardSystem;
```

---

## **Scoring Your Quiz**

### **How to Calculate Your Score:**

1. **Multiple Choice**: Count correct answers × 2 points each
2. **True/False**: Count correct answers × 1 point each  
3. **Practical Questions**: Rate each answer 0-5 points based on completeness and accuracy
4. **Bonus Challenge**: Rate 0-10 points based on code completeness and functionality

### **Grade Interpretation:**

- **Excellent (90%+)**: 47+ points - You have mastered dashboard design
- **Good (80-89%)**: 42-46 points - You understand the concepts well with minor gaps
- **Satisfactory (70-79%)**: 36-41 points - You have a basic understanding but need more practice
- **Needs Improvement (<70%)**: <36 points - Review the section material and retake the quiz

### **Next Steps Based on Your Score:**

- **90%+**: Excellent! You're ready for Section 14
- **80-89%**: Good work! Review any missed concepts before moving on
- **70-79%**: Spend more time on the hands-on exercises before proceeding
- **<70%**: Review the section material thoroughly and retake the quiz

---

**Great job completing Section 13! 🎉**

Ready for the next challenge? Move on to [Section 14: User Experience Design](../section14/README.md)!
