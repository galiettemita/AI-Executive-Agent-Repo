import "react-native-gesture-handler";
import React, { useEffect, useState } from "react";
import { NavigationContainer } from "@react-navigation/native";
import { createBottomTabNavigator } from "@react-navigation/bottom-tabs";
import { createNativeStackNavigator } from "@react-navigation/native-stack";
import { StatusBar } from "expo-status-bar";
import { useFonts, SpaceGrotesk_500Medium, SpaceGrotesk_700Bold } from "@expo-google-fonts/space-grotesk";

import { PairingScreen } from "./src/screens/PairingScreen";
import { HomeScreen } from "./src/screens/HomeScreen";
import { AlertsScreen } from "./src/screens/AlertsScreen";
import { SettingsScreen } from "./src/screens/SettingsScreen";
import { clearSession, loadSession, saveSession } from "./src/storage";

const Stack = createNativeStackNavigator();
const Tabs = createBottomTabNavigator();

function MainTabs({ userId, token, onSignOut }: { userId: string; token: string; onSignOut: () => void }) {
  return (
    <Tabs.Navigator screenOptions={{ headerShown: false }}>
      <Tabs.Screen name="Home">
        {() => <HomeScreen userId={userId} token={token} />}
      </Tabs.Screen>
      <Tabs.Screen name="Alerts">
        {() => <AlertsScreen userId={userId} token={token} />}
      </Tabs.Screen>
      <Tabs.Screen name="Settings">
        {() => <SettingsScreen userId={userId} onSignOut={onSignOut} />}
      </Tabs.Screen>
    </Tabs.Navigator>
  );
}

export default function App() {
  const [token, setToken] = useState<string | null>(null);
  const [userId, setUserId] = useState<string | null>(null);
  const [ready, setReady] = useState(false);

  const [fontsLoaded] = useFonts({
    SpaceGrotesk_500Medium,
    SpaceGrotesk_700Bold,
  });

  useEffect(() => {
    const hydrate = async () => {
      const session = await loadSession();
      setToken(session.token);
      setUserId(session.userId);
      setReady(true);
    };
    hydrate();
  }, []);

  const handlePaired = async (newToken: string, newUserId: string) => {
    await saveSession(newToken, newUserId);
    setToken(newToken);
    setUserId(newUserId);
  };

  const handleSignOut = async () => {
    await clearSession();
    setToken(null);
    setUserId(null);
  };

  if (!ready || !fontsLoaded) {
    return null;
  }

  return (
    <NavigationContainer>
      <StatusBar style="light" />
      <Stack.Navigator screenOptions={{ headerShown: false }}>
        {token && userId ? (
          <Stack.Screen name="Main">
            {() => <MainTabs userId={userId} token={token} onSignOut={handleSignOut} />}
          </Stack.Screen>
        ) : (
          <Stack.Screen name="Pairing">
            {() => <PairingScreen onPaired={handlePaired} />}
          </Stack.Screen>
        )}
      </Stack.Navigator>
    </NavigationContainer>
  );
}
