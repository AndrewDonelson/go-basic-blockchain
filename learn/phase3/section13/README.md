# Section 13: Dashboard Design

## 📊 Building Comprehensive Blockchain Monitoring Dashboards

Welcome to Section 13! This section focuses on creating powerful dashboards for monitoring blockchain networks, visualizing data, and managing system operations. You'll learn how to design comprehensive monitoring interfaces that provide real-time insights into blockchain performance, network health, and user activities.

### **What You'll Learn in This Section**

- Dashboard design principles and best practices
- Real-time data visualization with charts and graphs
- Interactive dashboard components and widgets
- Performance metrics and monitoring systems
- Data aggregation and filtering techniques
- Responsive dashboard layouts

### **Section Overview**

This section teaches you how to create professional dashboards that transform complex blockchain data into actionable insights. You'll build comprehensive monitoring interfaces that display real-time metrics, network status, transaction flows, and system performance in an intuitive and visually appealing way.

---

## 📈 Dashboard Design Principles

### **Core Design Principles**

#### **Information Hierarchy**
- **Primary metrics** prominently displayed
- **Secondary data** organized logically
- **Contextual information** easily accessible
- **Action items** clearly identified

#### **Visual Design**
- **Consistent color schemes** for data types
- **Clear typography** for readability
- **Proper spacing** for visual breathing room
- **Intuitive icons** for quick recognition

#### **User Experience**
- **Real-time updates** without page refresh
- **Interactive elements** for data exploration
- **Responsive design** for all screen sizes
- **Accessibility** for all users

### **Dashboard Types**

#### **Executive Dashboard**
- High-level overview of blockchain health
- Key performance indicators (KPIs)
- Network status and alerts
- Summary statistics

#### **Technical Dashboard**
- Detailed system metrics
- Network performance data
- Transaction processing stats
- Error rates and logs

#### **User Dashboard**
- Personal wallet information
- Transaction history
- Network participation
- Account statistics

---

## 🎨 Dashboard Architecture

### **Component Structure**

```
src/
├── components/
│   ├── dashboard/
│   │   ├── DashboardLayout.jsx
│   │   ├── MetricCard.jsx
│   │   ├── ChartWidget.jsx
│   │   ├── StatusIndicator.jsx
│   │   └── AlertPanel.jsx
│   ├── charts/
│   │   ├── LineChart.jsx
│   │   ├── BarChart.jsx
│   │   ├── PieChart.jsx
│   │   └── NetworkGraph.jsx
│   └── widgets/
│       ├── TransactionFeed.jsx
│       ├── BlockExplorer.jsx
│       ├── NetworkMap.jsx
│       └── PerformanceGauge.jsx
├── pages/
│   ├── ExecutiveDashboard.jsx
│   ├── TechnicalDashboard.jsx
│   └── UserDashboard.jsx
├── hooks/
│   ├── useDashboardData.js
│   ├── useRealTimeUpdates.js
│   └── useChartData.js
├── services/
│   ├── metricsService.js
│   ├── chartDataService.js
│   └── alertService.js
└── utils/
    ├── chartConfigs.js
    ├── colorSchemes.js
    └── formatters.js
```

### **Data Flow Architecture**

```javascript
// Dashboard data management
class DashboardDataManager {
  constructor() {
    this.subscribers = new Set();
    this.data = {
      metrics: {},
      charts: {},
      alerts: [],
      status: {}
    };
  }

  subscribe(callback) {
    this.subscribers.add(callback);
    return () => this.subscribers.delete(callback);
  }

  updateData(newData) {
    this.data = { ...this.data, ...newData };
    this.notifySubscribers();
  }

  notifySubscribers() {
    this.subscribers.forEach(callback => callback(this.data));
  }

  async fetchMetrics() {
    try {
      const response = await fetch('/api/v1/dashboard/metrics');
      const metrics = await response.json();
      this.updateData({ metrics });
    } catch (error) {
      console.error('Failed to fetch metrics:', error);
    }
  }

  async fetchChartData() {
    try {
      const response = await fetch('/api/v1/dashboard/charts');
      const charts = await response.json();
      this.updateData({ charts });
    } catch (error) {
      console.error('Failed to fetch chart data:', error);
    }
  }
}
```

---

## 📊 Core Dashboard Components

### **1. Executive Dashboard**

```jsx
import React, { useState, useEffect } from 'react';
import { useSelector } from 'react-redux';
import MetricCard from '../components/dashboard/MetricCard';
import ChartWidget from '../components/dashboard/ChartWidget';
import StatusIndicator from '../components/dashboard/StatusIndicator';
import AlertPanel from '../components/dashboard/AlertPanel';

const ExecutiveDashboard = () => {
  const { metrics, charts, alerts, status } = useSelector((state) => state.dashboard);
  const [timeRange, setTimeRange] = useState('24h');

  useEffect(() => {
    // Fetch dashboard data
    fetchDashboardData(timeRange);
  }, [timeRange]);

  const fetchDashboardData = async (range) => {
    // Fetch data based on time range
  };

  return (
    <div className="executive-dashboard">
      <div className="dashboard-header">
        <h1>Blockchain Network Overview</h1>
        <div className="time-range-selector">
          <select value={timeRange} onChange={(e) => setTimeRange(e.target.value)}>
            <option value="1h">Last Hour</option>
            <option value="24h">Last 24 Hours</option>
            <option value="7d">Last 7 Days</option>
            <option value="30d">Last 30 Days</option>
          </select>
        </div>
      </div>

      {/* Key Metrics Row */}
      <div className="metrics-grid">
        <MetricCard
          title="Total Transactions"
          value={metrics.totalTransactions}
          change={metrics.transactionChange}
          icon="transaction"
          color="blue"
        />
        <MetricCard
          title="Active Wallets"
          value={metrics.activeWallets}
          change={metrics.walletChange}
          icon="wallet"
          color="green"
        />
        <MetricCard
          title="Network Hash Rate"
          value={formatHashRate(metrics.hashRate)}
          change={metrics.hashRateChange}
          icon="mining"
          color="orange"
        />
        <MetricCard
          title="Block Height"
          value={metrics.blockHeight}
          change={metrics.blockChange}
          icon="block"
          color="purple"
        />
      </div>

      {/* Charts Row */}
      <div className="charts-grid">
        <ChartWidget
          title="Transaction Volume"
          type="line"
          data={charts.transactionVolume}
          height={300}
        />
        <ChartWidget
          title="Network Activity"
          type="bar"
          data={charts.networkActivity}
          height={300}
        />
      </div>

      {/* Status and Alerts */}
      <div className="status-section">
        <div className="status-grid">
          <StatusIndicator
            title="Network Status"
            status={status.network}
            details={status.networkDetails}
          />
          <StatusIndicator
            title="API Health"
            status={status.api}
            details={status.apiDetails}
          />
          <StatusIndicator
            title="Database"
            status={status.database}
            details={status.databaseDetails}
          />
        </div>

        <AlertPanel alerts={alerts} />
      </div>
    </div>
  );
};

const MetricCard = ({ title, value, change, icon, color }) => (
  <div className={`metric-card metric-card--${color}`}>
    <div className="metric-card__header">
      <div className={`metric-card__icon metric-card__icon--${icon}`} />
      <span className="metric-card__title">{title}</span>
    </div>
    <div className="metric-card__value">{value}</div>
    <div className={`metric-card__change metric-card__change--${change > 0 ? 'positive' : 'negative'}`}>
      {change > 0 ? '+' : ''}{change}%
    </div>
  </div>
);

const styles = `
.executive-dashboard {
  padding: 24px;
  background: #f8f9fa;
  min-height: 100vh;
}

.dashboard-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  gap: 20px;
  margin-bottom: 24px;
}

.charts-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(400px, 1fr));
  gap: 20px;
  margin-bottom: 24px;
}

.status-section {
  display: grid;
  grid-template-columns: 2fr 1fr;
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

.metric-card__header {
  display: flex;
  align-items: center;
  margin-bottom: 12px;
}

.metric-card__value {
  font-size: 32px;
  font-weight: bold;
  margin-bottom: 8px;
}

.metric-card__change {
  font-size: 14px;
  font-weight: 600;
}

.metric-card__change--positive {
  color: #10b981;
}

.metric-card__change--negative {
  color: #ef4444;
}
`;

export default ExecutiveDashboard;
```

### **2. Technical Dashboard**

```jsx
import React, { useState, useEffect } from 'react';
import { LineChart, BarChart, PieChart } from 'recharts';
import PerformanceGauge from '../components/widgets/PerformanceGauge';
import TransactionFeed from '../components/widgets/TransactionFeed';

const TechnicalDashboard = () => {
  const [performanceData, setPerformanceData] = useState({});
  const [systemMetrics, setSystemMetrics] = useState({});
  const [errorLogs, setErrorLogs] = useState([]);

  useEffect(() => {
    // Set up real-time data streams
    const performanceStream = new EventSource('/api/v1/dashboard/performance/stream');
    const metricsStream = new EventSource('/api/v1/dashboard/metrics/stream');

    performanceStream.onmessage = (event) => {
      const data = JSON.parse(event.data);
      setPerformanceData(data);
    };

    metricsStream.onmessage = (event) => {
      const data = JSON.parse(event.data);
      setSystemMetrics(data);
    };

    return () => {
      performanceStream.close();
      metricsStream.close();
    };
  }, []);

  return (
    <div className="technical-dashboard">
      <div className="dashboard-header">
        <h1>Technical Monitoring</h1>
        <div className="refresh-controls">
          <button onClick={() => window.location.reload()}>
            Refresh Data
          </button>
        </div>
      </div>

      {/* Performance Metrics */}
      <div className="performance-section">
        <h2>System Performance</h2>
        <div className="performance-grid">
          <PerformanceGauge
            title="CPU Usage"
            value={performanceData.cpu}
            max={100}
            color="#3b82f6"
          />
          <PerformanceGauge
            title="Memory Usage"
            value={performanceData.memory}
            max={100}
            color="#10b981"
          />
          <PerformanceGauge
            title="Disk Usage"
            value={performanceData.disk}
            max={100}
            color="#f59e0b"
          />
          <PerformanceGauge
            title="Network I/O"
            value={performanceData.network}
            max={100}
            color="#ef4444"
          />
        </div>
      </div>

      {/* Blockchain Metrics */}
      <div className="blockchain-metrics">
        <h2>Blockchain Performance</h2>
        <div className="metrics-grid">
          <div className="metric-panel">
            <h3>Transaction Processing</h3>
            <LineChart
              width={400}
              height={200}
              data={systemMetrics.transactionHistory}
            >
              {/* Chart configuration */}
            </LineChart>
          </div>

          <div className="metric-panel">
            <h3>Block Mining Rate</h3>
            <BarChart
              width={400}
              height={200}
              data={systemMetrics.miningRate}
            >
              {/* Chart configuration */}
            </BarChart>
          </div>

          <div className="metric-panel">
            <h3>Network Distribution</h3>
            <PieChart
              width={400}
              height={200}
              data={systemMetrics.networkDistribution}
            >
              {/* Chart configuration */}
            </PieChart>
          </div>
        </div>
      </div>

      {/* Real-time Transaction Feed */}
      <div className="transaction-feed-section">
        <h2>Live Transaction Feed</h2>
        <TransactionFeed />
      </div>

      {/* Error Logs */}
      <div className="error-logs-section">
        <h2>Error Logs</h2>
        <div className="error-logs">
          {errorLogs.map((error, index) => (
            <div key={index} className="error-log-item">
              <span className="error-timestamp">{error.timestamp}</span>
              <span className="error-level">{error.level}</span>
              <span className="error-message">{error.message}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};

const PerformanceGauge = ({ title, value, max, color }) => (
  <div className="performance-gauge">
    <h3>{title}</h3>
    <div className="gauge-container">
      <svg width="120" height="120" viewBox="0 0 120 120">
        <circle
          cx="60"
          cy="60"
          r="50"
          fill="none"
          stroke="#e5e7eb"
          strokeWidth="10"
        />
        <circle
          cx="60"
          cy="60"
          r="50"
          fill="none"
          stroke={color}
          strokeWidth="10"
          strokeDasharray={`${(value / max) * 314} 314`}
          transform="rotate(-90 60 60)"
        />
      </svg>
      <div className="gauge-value">{value}%</div>
    </div>
  </div>
);
```

### **3. User Dashboard**

```jsx
import React, { useState, useEffect } from 'react';
import { useSelector } from 'react-redux';
import WalletOverview from '../components/widgets/WalletOverview';
import TransactionHistory from '../components/widgets/TransactionHistory';
import NetworkParticipation from '../components/widgets/NetworkParticipation';

const UserDashboard = () => {
  const { user, wallets, transactions } = useSelector((state) => state.user);
  const [selectedWallet, setSelectedWallet] = useState(null);
  const [timeFilter, setTimeFilter] = useState('all');

  useEffect(() => {
    if (wallets.length > 0 && !selectedWallet) {
      setSelectedWallet(wallets[0]);
    }
  }, [wallets]);

  const filteredTransactions = transactions.filter(tx => {
    if (timeFilter === 'all') return true;
    if (timeFilter === 'today') {
      return new Date(tx.timestamp).toDateString() === new Date().toDateString();
    }
    if (timeFilter === 'week') {
      const weekAgo = new Date();
      weekAgo.setDate(weekAgo.getDate() - 7);
      return new Date(tx.timestamp) > weekAgo;
    }
    if (timeFilter === 'month') {
      const monthAgo = new Date();
      monthAgo.setMonth(monthAgo.getMonth() - 1);
      return new Date(tx.timestamp) > monthAgo;
    }
    return true;
  });

  return (
    <div className="user-dashboard">
      <div className="dashboard-header">
        <div className="user-info">
          <h1>Welcome back, {user.name}!</h1>
          <p>Here's your blockchain activity overview</p>
        </div>
        <div className="user-actions">
          <button className="btn-primary">Send Transaction</button>
          <button className="btn-secondary">Receive</button>
        </div>
      </div>

      {/* Wallet Overview */}
      <div className="wallet-section">
        <h2>My Wallets</h2>
        <div className="wallet-grid">
          {wallets.map((wallet) => (
            <WalletOverview
              key={wallet.id}
              wallet={wallet}
              isSelected={selectedWallet?.id === wallet.id}
              onSelect={() => setSelectedWallet(wallet)}
            />
          ))}
        </div>
      </div>

      {/* Transaction History */}
      <div className="transaction-section">
        <div className="section-header">
          <h2>Transaction History</h2>
          <div className="filters">
            <select value={timeFilter} onChange={(e) => setTimeFilter(e.target.value)}>
              <option value="all">All Time</option>
              <option value="today">Today</option>
              <option value="week">This Week</option>
              <option value="month">This Month</option>
            </select>
          </div>
        </div>
        <TransactionHistory transactions={filteredTransactions} />
      </div>

      {/* Network Participation */}
      <div className="participation-section">
        <h2>Network Participation</h2>
        <NetworkParticipation user={user} />
      </div>

      {/* Quick Stats */}
      <div className="quick-stats">
        <div className="stat-card">
          <h3>Total Balance</h3>
          <p className="stat-value">
            {wallets.reduce((total, wallet) => total + wallet.balance, 0)} BTC
          </p>
        </div>
        <div className="stat-card">
          <h3>Total Transactions</h3>
          <p className="stat-value">{transactions.length}</p>
        </div>
        <div className="stat-card">
          <h3>Active Wallets</h3>
          <p className="stat-value">{wallets.length}</p>
        </div>
        <div className="stat-card">
          <h3>Network Status</h3>
          <p className="stat-value">Connected</p>
        </div>
      </div>
    </div>
  );
};

const WalletOverview = ({ wallet, isSelected, onSelect }) => (
  <div
    className={`wallet-overview ${isSelected ? 'selected' : ''}`}
    onClick={onSelect}
  >
    <div className="wallet-header">
      <h3>{wallet.name}</h3>
      <span className="wallet-type">{wallet.type}</span>
    </div>
    <div className="wallet-balance">
      <span className="balance-amount">{wallet.balance} BTC</span>
      <span className="balance-usd">≈ ${wallet.balanceUSD}</span>
    </div>
    <div className="wallet-address">
      {wallet.address.substring(0, 20)}...
    </div>
    <div className="wallet-actions">
      <button className="btn-small">Send</button>
      <button className="btn-small">Receive</button>
    </div>
  </div>
);
```

---

## 📈 Data Visualization

### **Chart Configuration**

```javascript
// Chart configuration utilities
const chartConfigs = {
  lineChart: {
    margin: { top: 20, right: 30, left: 20, bottom: 5 },
    stroke: '#8884d8',
    strokeWidth: 2,
    dot: { fill: '#8884d8', strokeWidth: 2, r: 4 },
    activeDot: { r: 6 },
  },
  
  barChart: {
    margin: { top: 20, right: 30, left: 20, bottom: 5 },
    fill: '#8884d8',
    radius: [4, 4, 0, 0],
  },
  
  pieChart: {
    margin: { top: 20, right: 30, left: 20, bottom: 5 },
    innerRadius: 60,
    outerRadius: 80,
  },
};

// Color schemes for different data types
const colorSchemes = {
  transactions: ['#3b82f6', '#1d4ed8', '#1e40af'],
  blocks: ['#10b981', '#059669', '#047857'],
  wallets: ['#f59e0b', '#d97706', '#b45309'],
  errors: ['#ef4444', '#dc2626', '#b91c1c'],
};

// Data formatters
const formatters = {
  currency: (value) => new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD',
  }).format(value),
  
  percentage: (value) => `${value.toFixed(2)}%`,
  
  hashRate: (value) => {
    if (value >= 1e12) return `${(value / 1e12).toFixed(2)} TH/s`;
    if (value >= 1e9) return `${(value / 1e9).toFixed(2)} GH/s`;
    if (value >= 1e6) return `${(value / 1e6).toFixed(2)} MH/s`;
    if (value >= 1e3) return `${(value / 1e3).toFixed(2)} KH/s`;
    return `${value.toFixed(2)} H/s`;
  },
  
  timestamp: (value) => new Date(value).toLocaleString(),
};
```

### **Real-time Data Updates**

```javascript
// Real-time data hook
const useRealTimeData = (endpoint, interval = 5000) => {
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const response = await fetch(endpoint);
        const result = await response.json();
        setData(result);
        setError(null);
      } catch (err) {
        setError(err.message);
      } finally {
        setLoading(false);
      }
    };

    // Initial fetch
    fetchData();

    // Set up interval for real-time updates
    const intervalId = setInterval(fetchData, interval);

    return () => clearInterval(intervalId);
  }, [endpoint, interval]);

  return { data, loading, error };
};

// WebSocket for real-time updates
const useWebSocketData = (url) => {
  const [data, setData] = useState(null);

  useEffect(() => {
    const ws = new WebSocket(url);

    ws.onmessage = (event) => {
      const newData = JSON.parse(event.data);
      setData(newData);
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    return () => ws.close();
  }, [url]);

  return data;
};
```

---

## 🎯 Section Summary

In this section, you've learned:

✅ **Dashboard Design**: Professional dashboard layout and design principles
✅ **Data Visualization**: Charts, graphs, and interactive visualizations
✅ **Real-time Updates**: Live data streaming and WebSocket integration
✅ **Performance Monitoring**: System metrics and performance tracking
✅ **User Experience**: Intuitive and responsive dashboard interfaces
✅ **Data Management**: Efficient data handling and state management

### **Key Concepts Mastered**

1. **Dashboard Architecture**: Component-based dashboard design
2. **Data Visualization**: Chart.js, D3.js, and custom visualizations
3. **Real-time Data**: WebSocket and Server-Sent Events
4. **Performance Metrics**: System monitoring and alerting
5. **User Interface**: Responsive and accessible dashboard design
6. **Data Management**: State management and data flow

### **Next Steps**

1. Complete the hands-on exercises below
2. Take the quiz to test your understanding
3. Move on to [Section 14: User Experience Design](../section14/README.md)

---

## 🛠️ Hands-On Exercises

### **Exercise 1: Basic Dashboard Setup**
Create a basic dashboard with:
1. Dashboard layout and navigation
2. Basic metric cards
3. Simple charts and graphs
4. Responsive design

### **Exercise 2: Real-time Data Visualization**
Implement real-time features:
1. WebSocket data streaming
2. Live chart updates
3. Real-time metrics
4. Performance monitoring

### **Exercise 3: Interactive Dashboard**
Build interactive components:
1. Filterable data tables
2. Drill-down capabilities
3. Custom chart interactions
4. Dynamic data loading

### **Exercise 4: Performance Monitoring**
Create monitoring systems:
1. System health indicators
2. Performance gauges
3. Alert systems
4. Error tracking

### **Exercise 5: User Dashboard**
Design user-focused dashboards:
1. Personal wallet overview
2. Transaction history
3. Network participation
4. Account statistics

---

## 📝 Quiz

Ready to test your knowledge? Take the [Section 13 Quiz](./quiz.md) to verify your understanding of dashboard design.

---

**Excellent work! You've built comprehensive monitoring dashboards. You're ready to master user experience design in [Section 14](../section14/README.md)! 🚀**
