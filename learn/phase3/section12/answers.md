# Section 12 Quiz Answers

## 📋 Answer Key

Here are the correct answers and explanations for the Section 12 quiz.

---

## **Multiple Choice Questions**

### **Question 1: React Native Benefits**
**Answer: B) Cross-platform development with native performance**

**Explanation**: React Native allows developers to write code once and deploy to both iOS and Android while maintaining native performance, which is crucial for blockchain applications that require real-time updates and smooth user interactions.

### **Question 2: Mobile Wallet Security**
**Answer: B) Device keychain with biometric protection**

**Explanation**: The device keychain with biometric protection is the most secure method as it leverages the device's built-in security features and requires user authentication for access to sensitive data.

### **Question 3: Offline Functionality**
**Answer: B) Better user experience when offline**

**Explanation**: Offline-first architecture ensures users can continue using the app even without internet connectivity, providing a seamless experience and maintaining functionality when network conditions are poor.

### **Question 4: Push Notifications**
**Answer: B) Firebase Cloud Messaging**

**Explanation**: Firebase Cloud Messaging (FCM) is the most commonly used technology for push notifications in mobile apps, providing reliable delivery across iOS and Android platforms.

### **Question 5: Biometric Authentication**
**Answer: B) Fingerprint scanning**

**Explanation**: Fingerprint scanning is the most commonly used biometric method in mobile devices, offering a good balance of security and convenience.

### **Question 6: Mobile Performance**
**Answer: B) Memory management**

**Explanation**: Memory management is crucial for mobile app performance as mobile devices have limited RAM, and poor memory management can lead to crashes and poor user experience.

### **Question 7: Cross-Platform Development**
**Answer: C) 90-95%**

**Explanation**: React Native typically allows sharing 90-95% of code between iOS and Android platforms, with only platform-specific features requiring separate implementation.

### **Question 8: Mobile Navigation**
**Answer: D) All of the above**

**Explanation**: Mobile apps often use a combination of navigation patterns - tab navigation for main sections, stack navigation for detailed views, and drawer navigation for additional options.

---

## **True/False Questions**

### **Question 9**
**Answer: True**

**Explanation**: React Native can access native device features through native modules and libraries, including camera, biometrics, and other hardware features.

### **Question 10**
**Answer: False**

**Explanation**: Mobile wallets should support offline functionality to provide a better user experience and allow basic operations when internet connectivity is unavailable.

### **Question 11**
**Answer: False**

**Explanation**: Push notifications can be sent even when the app is not running, as they are handled by the operating system's notification system.

### **Question 12**
**Answer: True**

**Explanation**: Biometric authentication is generally more secure than password-based authentication as it's harder to replicate and provides additional security layers.

### **Question 13**
**Answer: True**

**Explanation**: Mobile apps should prioritize battery life as users expect their devices to last throughout the day, and poor battery optimization can lead to negative user experiences.

### **Question 14**
**Answer: False**

**Explanation**: While React Native provides good performance, it may not achieve exactly the same performance as native apps due to the JavaScript bridge overhead.

---

## **Practical Questions**

### **Question 15: Mobile Wallet Setup**

```jsx
import React, { useState, useEffect } from 'react';
import {
  View,
  Text,
  StyleSheet,
  TouchableOpacity,
  ScrollView,
  Alert,
} from 'react-native';
import { useSelector, useDispatch } from 'react-redux';
import { setWallets, setSelectedWallet } from '../store/walletSlice';
import { createWallet, getWallets } from '../services/wallet';

const MobileWallet = () => {
  const dispatch = useDispatch();
  const { wallets, selectedWallet } = useSelector((state) => state.wallet);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    loadWallets();
  }, []);

  const loadWallets = async () => {
    try {
      setLoading(true);
      const walletList = await getWallets();
      dispatch(setWallets(walletList));
      
      if (walletList.length > 0 && !selectedWallet) {
        dispatch(setSelectedWallet(walletList[0]));
      }
    } catch (error) {
      Alert.alert('Error', 'Failed to load wallets');
    } finally {
      setLoading(false);
    }
  };

  const handleCreateWallet = async () => {
    try {
      setLoading(true);
      const newWallet = await createWallet();
      dispatch(setWallets([...wallets, newWallet]));
      dispatch(setSelectedWallet(newWallet));
      Alert.alert('Success', 'New wallet created successfully!');
    } catch (error) {
      Alert.alert('Error', 'Failed to create wallet');
    } finally {
      setLoading(false);
    }
  };

  const handleSelectWallet = (wallet) => {
    dispatch(setSelectedWallet(wallet));
  };

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <Text style={styles.title}>My Wallets</Text>
        <TouchableOpacity
          style={styles.createButton}
          onPress={handleCreateWallet}
          disabled={loading}
        >
          <Text style={styles.createButtonText}>+ New Wallet</Text>
        </TouchableOpacity>
      </View>

      <ScrollView style={styles.walletList}>
        {wallets.map((wallet) => (
          <TouchableOpacity
            key={wallet.address}
            style={[
              styles.walletCard,
              selectedWallet?.address === wallet.address && styles.selectedCard
            ]}
            onPress={() => handleSelectWallet(wallet)}
          >
            <Text style={styles.walletName}>{wallet.name}</Text>
            <Text style={styles.walletAddress}>
              {wallet.address.substring(0, 20)}...
            </Text>
            <Text style={styles.walletBalance}>
              Balance: {wallet.balance} BTC
            </Text>
          </TouchableOpacity>
        ))}
      </ScrollView>

      {selectedWallet && (
        <View style={styles.selectedWallet}>
          <Text style={styles.selectedTitle}>Selected Wallet</Text>
          <Text style={styles.selectedAddress}>
            {selectedWallet.address.substring(0, 20)}...
          </Text>
          <Text style={styles.selectedBalance}>
            Balance: {selectedWallet.balance} BTC
          </Text>
        </View>
      )}
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f5f5f5',
  },
  header: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: 20,
    backgroundColor: 'white',
    borderBottomWidth: 1,
    borderBottomColor: '#e0e0e0',
  },
  title: {
    fontSize: 24,
    fontWeight: 'bold',
  },
  createButton: {
    backgroundColor: '#007AFF',
    paddingHorizontal: 16,
    paddingVertical: 8,
    borderRadius: 8,
  },
  createButtonText: {
    color: 'white',
    fontWeight: '600',
  },
  walletList: {
    flex: 1,
    padding: 16,
  },
  walletCard: {
    backgroundColor: 'white',
    padding: 16,
    borderRadius: 8,
    marginBottom: 12,
    borderWidth: 2,
    borderColor: 'transparent',
  },
  selectedCard: {
    borderColor: '#007AFF',
  },
  walletName: {
    fontSize: 18,
    fontWeight: '600',
    marginBottom: 4,
  },
  walletAddress: {
    fontSize: 14,
    color: '#666',
    marginBottom: 8,
  },
  walletBalance: {
    fontSize: 16,
    fontWeight: '600',
    color: '#007AFF',
  },
  selectedWallet: {
    backgroundColor: 'white',
    padding: 16,
    borderTopWidth: 1,
    borderTopColor: '#e0e0e0',
  },
  selectedTitle: {
    fontSize: 18,
    fontWeight: '600',
    marginBottom: 8,
  },
  selectedAddress: {
    fontSize: 14,
    color: '#666',
    marginBottom: 4,
  },
  selectedBalance: {
    fontSize: 16,
    fontWeight: '600',
    color: '#007AFF',
  },
});

export default MobileWallet;
```

### **Question 16: Offline Storage**

```jsx
import AsyncStorage from '@react-native-async-storage/async-storage';
import NetInfo from '@react-native-community/netinfo';

class OfflineStorage {
  static async saveTransaction(transaction) {
    try {
      const pendingTransactions = await this.getPendingTransactions();
      pendingTransactions.push({
        ...transaction,
        id: Date.now().toString(),
        status: 'pending',
        timestamp: new Date().toISOString(),
      });
      await AsyncStorage.setItem(
        'pendingTransactions',
        JSON.stringify(pendingTransactions)
      );
    } catch (error) {
      console.error('Failed to save pending transaction:', error);
    }
  }

  static async getPendingTransactions() {
    try {
      const transactions = await AsyncStorage.getItem('pendingTransactions');
      return transactions ? JSON.parse(transactions) : [];
    } catch (error) {
      console.error('Failed to get pending transactions:', error);
      return [];
    }
  }

  static async clearPendingTransactions() {
    try {
      await AsyncStorage.removeItem('pendingTransactions');
    } catch (error) {
      console.error('Failed to clear pending transactions:', error);
    }
  }

  static async syncPendingTransactions() {
    const isConnected = (await NetInfo.fetch()).isConnected;
    
    if (!isConnected) {
      return;
    }

    const pendingTransactions = await this.getPendingTransactions();
    
    for (const transaction of pendingTransactions) {
      try {
        await sendTransaction(transaction);
        // Remove from pending after successful sync
        const updatedPending = pendingTransactions.filter(tx => tx.id !== transaction.id);
        await AsyncStorage.setItem('pendingTransactions', JSON.stringify(updatedPending));
      } catch (error) {
        console.error('Failed to sync transaction:', error);
      }
    }
  }

  static async saveWalletData(walletData) {
    try {
      await AsyncStorage.setItem('walletData', JSON.stringify(walletData));
    } catch (error) {
      console.error('Failed to save wallet data:', error);
    }
  }

  static async getWalletData() {
    try {
      const data = await AsyncStorage.getItem('walletData');
      return data ? JSON.parse(data) : null;
    } catch (error) {
      console.error('Failed to get wallet data:', error);
      return null;
    }
  }
}

// Network status monitoring hook
const useNetworkStatus = () => {
  const [isConnected, setIsConnected] = useState(true);

  useEffect(() => {
    const unsubscribe = NetInfo.addEventListener((state) => {
      setIsConnected(state.isConnected);
      
      if (state.isConnected) {
        // Sync pending transactions when connection is restored
        OfflineStorage.syncPendingTransactions();
      }
    });

    return () => unsubscribe();
  }, []);

  return isConnected;
};

// Offline transaction component
const OfflineTransactionManager = () => {
  const [pendingTransactions, setPendingTransactions] = useState([]);
  const isConnected = useNetworkStatus();

  useEffect(() => {
    loadPendingTransactions();
  }, []);

  const loadPendingTransactions = async () => {
    const transactions = await OfflineStorage.getPendingTransactions();
    setPendingTransactions(transactions);
  };

  const handleSendTransaction = async (transactionData) => {
    if (isConnected) {
      try {
        await sendTransaction(transactionData);
      } catch (error) {
        // If online transaction fails, save for offline sync
        await OfflineStorage.saveTransaction(transactionData);
        setPendingTransactions(prev => [...prev, transactionData]);
      }
    } else {
      // Save for offline sync
      await OfflineStorage.saveTransaction(transactionData);
      setPendingTransactions(prev => [...prev, transactionData]);
    }
  };

  return (
    <View style={styles.container}>
      <View style={styles.statusBar}>
        <Text style={styles.statusText}>
          Status: {isConnected ? 'Online' : 'Offline'}
        </Text>
      </View>

      {pendingTransactions.length > 0 && (
        <View style={styles.pendingContainer}>
          <Text style={styles.pendingTitle}>
            Pending Transactions ({pendingTransactions.length})
          </Text>
          {pendingTransactions.map((tx) => (
            <View key={tx.id} style={styles.pendingTransaction}>
              <Text>To: {tx.to.substring(0, 20)}...</Text>
              <Text>Amount: {tx.amount} BTC</Text>
              <Text>Status: {tx.status}</Text>
            </View>
          ))}
        </View>
      )}
    </View>
  );
};
```

### **Question 17: Push Notifications**

```jsx
import PushNotification from 'react-native-push-notification';
import messaging from '@react-native-firebase/messaging';

class NotificationService {
  static init() {
    PushNotification.configure({
      onRegister: function (token) {
        console.log('TOKEN:', token);
        // Send token to server
        this.sendTokenToServer(token);
      },
      onNotification: function (notification) {
        console.log('NOTIFICATION:', notification);
        // Handle notification tap
        this.handleNotificationTap(notification);
      },
      permissions: {
        alert: true,
        badge: true,
        sound: true,
      },
      popInitialNotification: true,
      requestPermissions: true,
    });

    // Create notification channel
    PushNotification.createChannel(
      {
        channelId: 'blockchain-notifications',
        channelName: 'Blockchain Notifications',
        channelDescription: 'Notifications for blockchain activities',
        soundName: 'default',
        importance: 4,
        vibrate: true,
      },
      (created) => console.log(`Channel created: ${created}`)
    );
  }

  static async requestPermissions() {
    const authStatus = await messaging().requestPermission();
    const enabled =
      authStatus === messaging.AuthorizationStatus.AUTHORIZED ||
      authStatus === messaging.AuthorizationStatus.PROVISIONAL;

    if (enabled) {
      const fcmToken = await messaging().getToken();
      await this.sendTokenToServer(fcmToken);
      return fcmToken;
    }
  }

  static async sendTokenToServer(token) {
    try {
      await fetch('/api/v1/notifications/register', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ token }),
      });
    } catch (error) {
      console.error('Failed to send token to server:', error);
    }
  }

  static showLocalNotification(title, message, data = {}) {
    PushNotification.localNotification({
      title: title,
      message: message,
      data: data,
      channelId: 'blockchain-notifications',
      playSound: true,
      soundName: 'default',
      importance: 'high',
      priority: 'high',
    });
  }

  static showTransactionNotification(transaction) {
    const title = 'New Transaction';
    const message = `Received ${transaction.amount} BTC from ${transaction.from.substring(0, 10)}...`;
    
    this.showLocalNotification(title, message, {
      type: 'transaction',
      transactionId: transaction.id,
    });
  }

  static showBlockNotification(block) {
    const title = 'New Block Mined';
    const message = `Block #${block.index} with ${block.transactions.length} transactions`;
    
    this.showLocalNotification(title, message, {
      type: 'block',
      blockHash: block.hash,
    });
  }

  static handleNotificationTap(notification) {
    const { type, transactionId, blockHash } = notification.data;
    
    switch (type) {
      case 'transaction':
        // Navigate to transaction detail
        navigation.navigate('TransactionDetail', { transactionId });
        break;
      case 'block':
        // Navigate to block detail
        navigation.navigate('BlockDetail', { blockHash });
        break;
      default:
        // Navigate to main screen
        navigation.navigate('Home');
    }
  }

  static scheduleNotification(title, message, date, data = {}) {
    PushNotification.localNotificationSchedule({
      title: title,
      message: message,
      date: date,
      data: data,
      channelId: 'blockchain-notifications',
      playSound: true,
      soundName: 'default',
    });
  }
}

// Notification component
const NotificationManager = () => {
  const [notifications, setNotifications] = useState([]);

  useEffect(() => {
    NotificationService.init();
    NotificationService.requestPermissions();
    
    // Listen for incoming messages
    const unsubscribe = messaging().onMessage(async remoteMessage => {
      console.log('Received foreground message:', remoteMessage);
      
      // Show local notification
      NotificationService.showLocalNotification(
        remoteMessage.notification.title,
        remoteMessage.notification.body,
        remoteMessage.data
      );
    });

    return unsubscribe;
  }, []);

  const handleTransactionNotification = (transaction) => {
    NotificationService.showTransactionNotification(transaction);
  };

  const handleBlockNotification = (block) => {
    NotificationService.showBlockNotification(block);
  };

  return (
    <View style={styles.container}>
      <Text style={styles.title}>Notification Settings</Text>
      
      <TouchableOpacity
        style={styles.button}
        onPress={() => NotificationService.requestPermissions()}
      >
        <Text style={styles.buttonText}>Request Permissions</Text>
      </TouchableOpacity>

      <TouchableOpacity
        style={styles.button}
        onPress={() => NotificationService.showLocalNotification(
          'Test Notification',
          'This is a test notification'
        )}
      >
        <Text style={styles.buttonText}>Test Notification</Text>
      </TouchableOpacity>
    </View>
  );
};
```

### **Question 18: Biometric Authentication**

```jsx
import ReactNativeBiometrics from 'react-native-biometrics';

class BiometricService {
  static async isBiometricAvailable() {
    try {
      const { available, biometryType } = await ReactNativeBiometrics.isSensorAvailable();
      return { available, biometryType };
    } catch (error) {
      console.error('Biometric check failed:', error);
      return { available: false, biometryType: null };
    }
  }

  static async authenticate(reason = 'Please authenticate to access your wallet') {
    try {
      const { success } = await ReactNativeBiometrics.simplePrompt({
        promptMessage: reason,
        cancelButtonText: 'Cancel',
        fallbackPromptMessage: 'Use passcode',
      });
      return success;
    } catch (error) {
      console.error('Biometric authentication failed:', error);
      return false;
    }
  }

  static async createKeys() {
    try {
      const { keysExist } = await ReactNativeBiometrics.biometricKeysExist();
      
      if (!keysExist) {
        const { publicKey } = await ReactNativeBiometrics.createKeys();
        return publicKey;
      }
    } catch (error) {
      console.error('Failed to create biometric keys:', error);
    }
  }

  static async signTransaction(transactionData) {
    try {
      const { signature } = await ReactNativeBiometrics.createSignature({
        promptMessage: 'Sign transaction with biometric',
        payload: JSON.stringify(transactionData),
        cancelButtonText: 'Cancel',
      });
      return signature;
    } catch (error) {
      console.error('Failed to sign transaction:', error);
      throw error;
    }
  }

  static async encryptData(data) {
    try {
      const { encrypted } = await ReactNativeBiometrics.createSignature({
        promptMessage: 'Encrypt data with biometric',
        payload: JSON.stringify(data),
        cancelButtonText: 'Cancel',
      });
      return encrypted;
    } catch (error) {
      console.error('Failed to encrypt data:', error);
      throw error;
    }
  }
}

// Biometric authentication component
const BiometricAuth = ({ onSuccess, onFailure, children }) => {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [biometricAvailable, setBiometricAvailable] = useState(false);
  const [biometryType, setBiometryType] = useState(null);

  useEffect(() => {
    checkBiometricAvailability();
  }, []);

  const checkBiometricAvailability = async () => {
    const { available, biometryType } = await BiometricService.isBiometricAvailable();
    setBiometricAvailable(available);
    setBiometryType(biometryType);
  };

  const handleAuthenticate = async () => {
    try {
      const success = await BiometricService.authenticate(
        'Please authenticate to access your wallet'
      );
      
      if (success) {
        setIsAuthenticated(true);
        onSuccess && onSuccess();
      } else {
        onFailure && onFailure('Authentication failed');
      }
    } catch (error) {
      onFailure && onFailure('Authentication error');
    }
  };

  if (!biometricAvailable) {
    return (
      <View style={styles.container}>
        <Text style={styles.title}>Biometric Authentication Not Available</Text>
        <Text style={styles.subtitle}>
          Please use passcode or password authentication
        </Text>
        {children}
      </View>
    );
  }

  if (isAuthenticated) {
    return children;
  }

  return (
    <View style={styles.container}>
      <View style={styles.authContainer}>
        <Text style={styles.title}>Biometric Authentication</Text>
        <Text style={styles.subtitle}>
          Use your {biometryType} to access your wallet
        </Text>
        
        <TouchableOpacity
          style={styles.authButton}
          onPress={handleAuthenticate}
        >
          <Text style={styles.authButtonText}>
            Authenticate with {biometryType}
          </Text>
        </TouchableOpacity>

        <TouchableOpacity
          style={styles.fallbackButton}
          onPress={() => onFailure && onFailure('Fallback to passcode')}
        >
          <Text style={styles.fallbackButtonText}>Use Passcode</Text>
        </TouchableOpacity>
      </View>
    </View>
  );
};

// Secure transaction component
const SecureTransaction = ({ transaction, onSend }) => {
  const [loading, setLoading] = useState(false);

  const handleSecureSend = async () => {
    try {
      setLoading(true);
      
      // Sign transaction with biometric
      const signature = await BiometricService.signTransaction(transaction);
      
      // Add signature to transaction
      const signedTransaction = {
        ...transaction,
        signature,
      };
      
      // Send transaction
      await onSend(signedTransaction);
      
    } catch (error) {
      console.error('Secure transaction failed:', error);
      Alert.alert('Error', 'Transaction failed. Please try again.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <View style={styles.container}>
      <Text style={styles.title}>Secure Transaction</Text>
      
      <View style={styles.transactionDetails}>
        <Text>To: {transaction.to.substring(0, 20)}...</Text>
        <Text>Amount: {transaction.amount} BTC</Text>
        <Text>Fee: {transaction.fee} BTC</Text>
      </View>

      <TouchableOpacity
        style={[styles.sendButton, loading && styles.sendButtonDisabled]}
        onPress={handleSecureSend}
        disabled={loading}
      >
        <Text style={styles.sendButtonText}>
          {loading ? 'Signing...' : 'Sign & Send Transaction'}
        </Text>
      </TouchableOpacity>
    </View>
  );
};
```

---

## **Bonus Challenge: Complete Mobile App**

```jsx
// Complete mobile blockchain app
import React, { useState, useEffect } from 'react';
import { NavigationContainer } from '@react-navigation/native';
import { createBottomTabNavigator } from '@react-navigation/bottom-tabs';
import { createStackNavigator } from '@react-navigation/stack';
import { Provider } from 'react-redux';
import { store } from './store';
import { BiometricAuth } from './components/BiometricAuth';
import { NotificationManager } from './components/NotificationManager';
import { OfflineTransactionManager } from './components/OfflineTransactionManager';

const Tab = createBottomTabNavigator();
const Stack = createStackNavigator();

// Main app component
const App = () => {
  const [isAuthenticated, setIsAuthenticated] = useState(false);

  return (
    <Provider store={store}>
      <BiometricAuth
        onSuccess={() => setIsAuthenticated(true)}
        onFailure={() => setIsAuthenticated(false)}
      >
        <NavigationContainer>
          <Tab.Navigator>
            <Tab.Screen name="Home" component={HomeScreen} />
            <Tab.Screen name="Wallet" component={WalletScreen} />
            <Tab.Screen name="Send" component={SendScreen} />
            <Tab.Screen name="Receive" component={ReceiveScreen} />
            <Tab.Screen name="Transactions" component={TransactionScreen} />
          </Tab.Navigator>
        </NavigationContainer>
        
        <NotificationManager />
        <OfflineTransactionManager />
      </BiometricAuth>
    </Provider>
  );
};

// Home screen with real-time updates
const HomeScreen = () => {
  const { selectedWallet, transactions } = useSelector((state) => state.wallet);
  const [networkStatus, setNetworkStatus] = useState({});

  useEffect(() => {
    // WebSocket connection for real-time updates
    const ws = new WebSocket('ws://localhost:8080/ws');
    
    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      handleRealTimeUpdate(data);
    };

    return () => ws.close();
  }, []);

  const handleRealTimeUpdate = (data) => {
    switch (data.type) {
      case 'new_transaction':
        if (data.transaction.to === selectedWallet?.address) {
          NotificationService.showTransactionNotification(data.transaction);
        }
        break;
      case 'new_block':
        NotificationService.showBlockNotification(data.block);
        break;
      case 'network_status':
        setNetworkStatus(data.status);
        break;
    }
  };

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <Text style={styles.title}>Blockchain Wallet</Text>
        <Text style={styles.subtitle}>
          Network: {networkStatus.status || 'Connecting...'}
        </Text>
      </View>

      {selectedWallet && (
        <View style={styles.walletCard}>
          <Text style={styles.balanceTitle}>Balance</Text>
          <Text style={styles.balanceAmount}>
            {selectedWallet.balance} BTC
          </Text>
          <Text style={styles.walletAddress}>
            {selectedWallet.address.substring(0, 20)}...
          </Text>
        </View>
      )}

      <View style={styles.quickActions}>
        <TouchableOpacity
          style={styles.actionButton}
          onPress={() => navigation.navigate('Send')}
        >
          <Text style={styles.actionButtonText}>Send</Text>
        </TouchableOpacity>
        
        <TouchableOpacity
          style={styles.actionButton}
          onPress={() => navigation.navigate('Receive')}
        >
          <Text style={styles.actionButtonText}>Receive</Text>
        </TouchableOpacity>
      </View>

      <View style={styles.recentTransactions}>
        <Text style={styles.sectionTitle}>Recent Transactions</Text>
        {transactions.slice(0, 5).map((tx) => (
          <TransactionItem key={tx.id} transaction={tx} />
        ))}
      </View>
    </View>
  );
};

export default App;
```

---

## **Scoring Your Quiz**

### **How to Calculate Your Score:**

1. **Multiple Choice**: Count correct answers × 2 points each
2. **True/False**: Count correct answers × 1 point each  
3. **Practical Questions**: Rate each answer 0-5 points based on completeness and accuracy
4. **Bonus Challenge**: Rate 0-10 points based on code completeness and functionality

### **Grade Interpretation:**

- **Excellent (90%+)**: 47+ points - You have mastered mobile app development
- **Good (80-89%)**: 42-46 points - You understand the concepts well with minor gaps
- **Satisfactory (70-79%)**: 36-41 points - You have a basic understanding but need more practice
- **Needs Improvement (<70%)**: <36 points - Review the section material and retake the quiz

### **Next Steps Based on Your Score:**

- **90%+**: Excellent! You're ready for Section 13
- **80-89%**: Good work! Review any missed concepts before moving on
- **70-79%**: Spend more time on the hands-on exercises before proceeding
- **<70%**: Review the section material thoroughly and retake the quiz

---

**Great job completing Section 12! 🎉**

Ready for the next challenge? Move on to [Section 13: Dashboard Design](../section13/README.md)!
