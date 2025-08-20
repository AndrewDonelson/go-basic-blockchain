# Section 12: Mobile App Development

## 📱 Building Cross-Platform Blockchain Mobile Applications

Welcome to Section 12! This section focuses on creating mobile applications for blockchain systems using React Native. You'll learn how to build cross-platform mobile apps that provide blockchain functionality on-the-go, including wallet management, transaction capabilities, and real-time blockchain monitoring.

### **What You'll Learn in This Section**

- Cross-platform mobile development with React Native
- Blockchain wallet integration for mobile devices
- Offline functionality and data synchronization
- Push notifications and real-time alerts
- Mobile-specific UI/UX design patterns
- Native device integration (camera, biometrics, etc.)

### **Section Overview**

This section teaches you how to create mobile applications that bring blockchain functionality to users' pockets. You'll build a complete mobile blockchain wallet with transaction capabilities, real-time updates, and offline functionality that works seamlessly across iOS and Android platforms.

---

## 📱 Mobile Development Stack

### **React Native Framework**

#### **Core Features**
- **Cross-platform development** for iOS and Android
- **Native performance** with JavaScript bridge
- **Hot reloading** for rapid development
- **Rich ecosystem** of libraries and tools

#### **Key Libraries**
- **React Navigation**: Navigation between screens
- **AsyncStorage**: Local data persistence
- **React Native Elements**: UI component library
- **React Native Vector Icons**: Icon library
- **React Native Crypto**: Cryptographic operations

### **Mobile-Specific Considerations**

#### **Performance Optimization**
- **Memory management** for mobile devices
- **Battery optimization** for blockchain operations
- **Network efficiency** for data synchronization
- **Offline-first architecture** for reliability

#### **Security Features**
- **Biometric authentication** (Touch ID, Face ID)
- **Secure key storage** using device keychain
- **Encrypted local storage** for sensitive data
- **Certificate pinning** for API security

---

## 🏗️ Mobile App Architecture

### **Project Structure**

```
src/
├── components/
│   ├── common/
│   │   ├── Button.js
│   │   ├── Input.js
│   │   └── Loading.js
│   ├── wallet/
│   │   ├── WalletCard.js
│   │   ├── TransactionItem.js
│   │   └── QRScanner.js
│   └── blockchain/
│       ├── BlockExplorer.js
│       ├── TransactionList.js
│       └── NetworkStatus.js
├── screens/
│   ├── HomeScreen.js
│   ├── WalletScreen.js
│   ├── SendScreen.js
│   ├── ReceiveScreen.js
│   ├── TransactionScreen.js
│   └── SettingsScreen.js
├── navigation/
│   ├── AppNavigator.js
│   ├── TabNavigator.js
│   └── StackNavigator.js
├── services/
│   ├── api.js
│   ├── blockchain.js
│   ├── wallet.js
│   └── notifications.js
├── store/
│   ├── index.js
│   ├── walletSlice.js
│   └── blockchainSlice.js
└── utils/
    ├── constants.js
    ├── helpers.js
    └── security.js
```

### **State Management**

```javascript
// Redux store configuration
import { configureStore } from '@reduxjs/toolkit';
import walletReducer from './walletSlice';
import blockchainReducer from './blockchainSlice';

export const store = configureStore({
  reducer: {
    wallet: walletReducer,
    blockchain: blockchainReducer,
  },
  middleware: (getDefaultMiddleware) =>
    getDefaultMiddleware({
      serializableCheck: {
        ignoredActions: ['wallet/setPrivateKey'],
      },
    }),
});

// Wallet slice for state management
import { createSlice } from '@reduxjs/toolkit';

const walletSlice = createSlice({
  name: 'wallet',
  initialState: {
    wallets: [],
    selectedWallet: null,
    balance: 0,
    transactions: [],
    loading: false,
    error: null,
  },
  reducers: {
    setWallets: (state, action) => {
      state.wallets = action.payload;
    },
    setSelectedWallet: (state, action) => {
      state.selectedWallet = action.payload;
    },
    setBalance: (state, action) => {
      state.balance = action.payload;
    },
    addTransaction: (state, action) => {
      state.transactions.unshift(action.payload);
    },
    setLoading: (state, action) => {
      state.loading = action.payload;
    },
    setError: (state, action) => {
      state.error = action.payload;
    },
  },
});

export const {
  setWallets,
  setSelectedWallet,
  setBalance,
  addTransaction,
  setLoading,
  setError,
} = walletSlice.actions;

export default walletSlice.reducer;
```

---

## 🎯 Core Mobile Components

### **1. Wallet Management Screen**

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
import WalletCard from '../components/wallet/WalletCard';
import { createWallet, getWallets } from '../services/wallet';

const WalletScreen = ({ navigation }) => {
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
          <WalletCard
            key={wallet.address}
            wallet={wallet}
            isSelected={selectedWallet?.address === wallet.address}
            onPress={() => handleSelectWallet(wallet)}
          />
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

export default WalletScreen;
```

### **2. Send Transaction Screen**

```jsx
import React, { useState } from 'react';
import {
  View,
  Text,
  StyleSheet,
  TextInput,
  TouchableOpacity,
  Alert,
  KeyboardAvoidingView,
  Platform,
} from 'react-native';
import { useSelector } from 'react-redux';
import QRCodeScanner from 'react-native-qrcode-scanner';
import { sendTransaction } from '../services/blockchain';
import { addTransaction } from '../store/walletSlice';

const SendScreen = ({ navigation }) => {
  const dispatch = useDispatch();
  const { selectedWallet } = useSelector((state) => state.wallet);
  const [recipient, setRecipient] = useState('');
  const [amount, setAmount] = useState('');
  const [note, setNote] = useState('');
  const [loading, setLoading] = useState(false);
  const [showScanner, setShowScanner] = useState(false);

  const handleSend = async () => {
    if (!recipient || !amount) {
      Alert.alert('Error', 'Please fill in all required fields');
      return;
    }

    if (parseFloat(amount) <= 0) {
      Alert.alert('Error', 'Amount must be greater than 0');
      return;
    }

    if (parseFloat(amount) > selectedWallet.balance) {
      Alert.alert('Error', 'Insufficient balance');
      return;
    }

    try {
      setLoading(true);
      
      const transaction = await sendTransaction({
        from: selectedWallet.address,
        to: recipient,
        amount: parseFloat(amount),
        note: note,
      });

      dispatch(addTransaction(transaction));
      
      Alert.alert(
        'Success',
        'Transaction sent successfully!',
        [
          {
            text: 'OK',
            onPress: () => navigation.navigate('Home'),
          },
        ]
      );
    } catch (error) {
      Alert.alert('Error', 'Failed to send transaction');
    } finally {
      setLoading(false);
    }
  };

  const handleQRScan = (event) => {
    setRecipient(event.data);
    setShowScanner(false);
  };

  return (
    <KeyboardAvoidingView
      style={styles.container}
      behavior={Platform.OS === 'ios' ? 'padding' : 'height'}
    >
      <View style={styles.content}>
        <Text style={styles.title}>Send Bitcoin</Text>

        <View style={styles.inputContainer}>
          <Text style={styles.label}>Recipient Address</Text>
          <View style={styles.recipientContainer}>
            <TextInput
              style={styles.input}
              value={recipient}
              onChangeText={setRecipient}
              placeholder="Enter recipient address"
              autoCapitalize="none"
            />
            <TouchableOpacity
              style={styles.scanButton}
              onPress={() => setShowScanner(true)}
            >
              <Text style={styles.scanButtonText}>Scan</Text>
            </TouchableOpacity>
          </View>
        </View>

        <View style={styles.inputContainer}>
          <Text style={styles.label}>Amount (BTC)</Text>
          <TextInput
            style={styles.input}
            value={amount}
            onChangeText={setAmount}
            placeholder="0.00000000"
            keyboardType="numeric"
          />
        </View>

        <View style={styles.inputContainer}>
          <Text style={styles.label}>Note (Optional)</Text>
          <TextInput
            style={[styles.input, styles.noteInput]}
            value={note}
            onChangeText={setNote}
            placeholder="Add a note to this transaction"
            multiline
          />
        </View>

        <View style={styles.balanceContainer}>
          <Text style={styles.balanceLabel}>Available Balance:</Text>
          <Text style={styles.balanceAmount}>
            {selectedWallet.balance} BTC
          </Text>
        </View>

        <TouchableOpacity
          style={[styles.sendButton, loading && styles.sendButtonDisabled]}
          onPress={handleSend}
          disabled={loading}
        >
          <Text style={styles.sendButtonText}>
            {loading ? 'Sending...' : 'Send Transaction'}
          </Text>
        </TouchableOpacity>
      </View>

      {showScanner && (
        <QRCodeScanner
          onRead={handleQRScan}
          topContent={<Text>Scan QR Code</Text>}
          bottomContent={
            <TouchableOpacity onPress={() => setShowScanner(false)}>
              <Text>Cancel</Text>
            </TouchableOpacity>
          }
        />
      )}
    </KeyboardAvoidingView>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f5f5f5',
  },
  content: {
    flex: 1,
    padding: 20,
  },
  title: {
    fontSize: 24,
    fontWeight: 'bold',
    marginBottom: 20,
  },
  inputContainer: {
    marginBottom: 20,
  },
  label: {
    fontSize: 16,
    fontWeight: '600',
    marginBottom: 8,
  },
  input: {
    backgroundColor: 'white',
    borderWidth: 1,
    borderColor: '#e0e0e0',
    borderRadius: 8,
    padding: 12,
    fontSize: 16,
  },
  recipientContainer: {
    flexDirection: 'row',
    alignItems: 'center',
  },
  scanButton: {
    backgroundColor: '#007AFF',
    paddingHorizontal: 16,
    paddingVertical: 12,
    borderRadius: 8,
    marginLeft: 8,
  },
  scanButtonText: {
    color: 'white',
    fontWeight: '600',
  },
  noteInput: {
    height: 80,
    textAlignVertical: 'top',
  },
  balanceContainer: {
    backgroundColor: 'white',
    padding: 16,
    borderRadius: 8,
    marginBottom: 20,
  },
  balanceLabel: {
    fontSize: 14,
    color: '#666',
  },
  balanceAmount: {
    fontSize: 18,
    fontWeight: 'bold',
    color: '#007AFF',
  },
  sendButton: {
    backgroundColor: '#007AFF',
    paddingVertical: 16,
    borderRadius: 8,
    alignItems: 'center',
  },
  sendButtonDisabled: {
    backgroundColor: '#ccc',
  },
  sendButtonText: {
    color: 'white',
    fontSize: 18,
    fontWeight: '600',
  },
});

export default SendScreen;
```

### **3. Transaction History Screen**

```jsx
import React, { useState, useEffect } from 'react';
import {
  View,
  Text,
  StyleSheet,
  FlatList,
  TouchableOpacity,
  RefreshControl,
} from 'react-native';
import { useSelector } from 'react-redux';
import TransactionItem from '../components/wallet/TransactionItem';
import { getTransactions } from '../services/blockchain';

const TransactionScreen = ({ navigation }) => {
  const { selectedWallet, transactions } = useSelector((state) => state.wallet);
  const [refreshing, setRefreshing] = useState(false);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (selectedWallet) {
      loadTransactions();
    }
  }, [selectedWallet]);

  const loadTransactions = async () => {
    try {
      setLoading(true);
      const txList = await getTransactions(selectedWallet.address);
      // Update transactions in store
    } catch (error) {
      console.error('Failed to load transactions:', error);
    } finally {
      setLoading(false);
    }
  };

  const onRefresh = async () => {
    setRefreshing(true);
    await loadTransactions();
    setRefreshing(false);
  };

  const renderTransaction = ({ item }) => (
    <TransactionItem
      transaction={item}
      onPress={() => navigation.navigate('TransactionDetail', { transaction: item })}
    />
  );

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <Text style={styles.title}>Transaction History</Text>
        <Text style={styles.subtitle}>
          {selectedWallet?.address.substring(0, 20)}...
        </Text>
      </View>

      <FlatList
        data={transactions}
        renderItem={renderTransaction}
        keyExtractor={(item) => item.id}
        refreshControl={
          <RefreshControl refreshing={refreshing} onRefresh={onRefresh} />
        }
        ListEmptyComponent={
          <View style={styles.emptyContainer}>
            <Text style={styles.emptyText}>No transactions yet</Text>
            <Text style={styles.emptySubtext}>
              Your transaction history will appear here
            </Text>
          </View>
        }
      />
    </View>
  );
};

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#f5f5f5',
  },
  header: {
    backgroundColor: 'white',
    padding: 20,
    borderBottomWidth: 1,
    borderBottomColor: '#e0e0e0',
  },
  title: {
    fontSize: 24,
    fontWeight: 'bold',
  },
  subtitle: {
    fontSize: 14,
    color: '#666',
    marginTop: 4,
  },
  emptyContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    padding: 40,
  },
  emptyText: {
    fontSize: 18,
    fontWeight: '600',
    color: '#666',
    marginBottom: 8,
  },
  emptySubtext: {
    fontSize: 14,
    color: '#999',
    textAlign: 'center',
  },
});

export default TransactionScreen;
```

---

## 🔄 Offline Functionality

### **Offline-First Architecture**

```javascript
// Offline storage service
import AsyncStorage from '@react-native-async-storage/async-storage';
import NetInfo from '@react-native-community/netinfo';

class OfflineStorage {
  static async saveTransaction(transaction) {
    try {
      const pendingTransactions = await this.getPendingTransactions();
      pendingTransactions.push(transaction);
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
      } catch (error) {
        console.error('Failed to sync transaction:', error);
      }
    }

    await this.clearPendingTransactions();
  }
}

// Network status monitoring
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
```

---

## 🔔 Push Notifications

### **Notification Service**

```javascript
import PushNotification from 'react-native-push-notification';
import messaging from '@react-native-firebase/messaging';

class NotificationService {
  static init() {
    PushNotification.configure({
      onRegister: function (token) {
        console.log('TOKEN:', token);
      },
      onNotification: function (notification) {
        console.log('NOTIFICATION:', notification);
      },
      permissions: {
        alert: true,
        badge: true,
        sound: true,
      },
      popInitialNotification: true,
      requestPermissions: true,
    });
  }

  static async requestPermissions() {
    const authStatus = await messaging().requestPermission();
    const enabled =
      authStatus === messaging.AuthorizationStatus.AUTHORIZED ||
      authStatus === messaging.AuthorizationStatus.PROVISIONAL;

    if (enabled) {
      const fcmToken = await messaging().getToken();
      // Send FCM token to server
      return fcmToken;
    }
  }

  static showLocalNotification(title, message, data = {}) {
    PushNotification.localNotification({
      title: title,
      message: message,
      data: data,
      channelId: 'blockchain-notifications',
    });
  }

  static scheduleNotification(title, message, date, data = {}) {
    PushNotification.localNotificationSchedule({
      title: title,
      message: message,
      date: date,
      data: data,
      channelId: 'blockchain-notifications',
    });
  }
}

// Notification channels
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
```

---

## 🔐 Security Features

### **Biometric Authentication**

```javascript
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
        promptMessage: 'Sign transaction',
        payload: JSON.stringify(transactionData),
      });
      return signature;
    } catch (error) {
      console.error('Failed to sign transaction:', error);
      throw error;
    }
  }
}
```

---

## 🎯 Section Summary

In this section, you've learned:

✅ **Cross-Platform Development**: React Native for iOS and Android
✅ **Mobile Wallet Integration**: Complete wallet management system
✅ **Offline Functionality**: Offline-first architecture with sync
✅ **Push Notifications**: Real-time alerts and updates
✅ **Security Features**: Biometric authentication and secure storage
✅ **Mobile UI/UX**: Touch-friendly, mobile-optimized interfaces

### **Key Concepts Mastered**

1. **React Native Development**: Cross-platform mobile app development
2. **Mobile Wallet**: Complete blockchain wallet functionality
3. **Offline Architecture**: Data persistence and synchronization
4. **Push Notifications**: Real-time communication with users
5. **Mobile Security**: Biometric authentication and secure storage
6. **Mobile UX**: Touch-friendly, responsive mobile interfaces

### **Next Steps**

1. Complete the hands-on exercises below
2. Take the quiz to test your understanding
3. Move on to [Section 13: Dashboard Design](../section13/README.md)

---

## 🛠️ Hands-On Exercises

### **Exercise 1: React Native Setup**
Set up a React Native project with:
1. React Native CLI or Expo setup
2. Navigation configuration
3. State management with Redux
4. Basic component structure

### **Exercise 2: Mobile Wallet**
Create a mobile wallet that:
1. Manages multiple wallets
2. Displays balances and transactions
3. Handles send/receive functionality
4. Implements QR code scanning

### **Exercise 3: Offline Functionality**
Implement offline features:
1. Local data storage with AsyncStorage
2. Offline transaction queuing
3. Network status monitoring
4. Data synchronization

### **Exercise 4: Push Notifications**
Add notification system:
1. Firebase Cloud Messaging setup
2. Local notification handling
3. Transaction alerts
4. Network status notifications

### **Exercise 5: Security Features**
Implement security:
1. Biometric authentication
2. Secure key storage
3. Transaction signing
4. App security measures

---

## 📝 Quiz

Ready to test your knowledge? Take the [Section 12 Quiz](./quiz.md) to verify your understanding of mobile app development.

---

**Excellent work! You've built a mobile blockchain wallet. You're ready to create comprehensive dashboards in [Section 13](../section13/README.md)! 🚀**
