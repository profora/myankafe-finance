import { Ionicons } from "@expo/vector-icons";
import { Tabs } from "expo-router";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { colors } from "../../src/theme";

export default function TabsLayout() {
  const insets = useSafeAreaInsets();
  const tabPad = Math.max(insets.bottom, 8);
  return (
    <Tabs
      backBehavior="none"
      screenOptions={{
        headerShown: false,
        tabBarActiveTintColor: colors.accent,
        tabBarInactiveTintColor: colors.muted,
        tabBarStyle: {
          minHeight: 56 + tabPad,
          paddingBottom: tabPad,
          height: 56 + tabPad,
        },
      }}
    >
      <Tabs.Screen name="index" options={{ title: "Home", tabBarButtonTestID: "Tab Home", tabBarIcon: ({ color, size }) => <Ionicons name="home-outline" color={color} size={size} /> }} />
      <Tabs.Screen name="transactions" options={{ title: "Transactions", tabBarButtonTestID: "Tab Transactions", tabBarIcon: ({ color, size }) => <Ionicons name="list-outline" color={color} size={size} /> }} />
      <Tabs.Screen name="reports" options={{ title: "Reports", tabBarButtonTestID: "Tab Reports", tabBarIcon: ({ color, size }) => <Ionicons name="bar-chart-outline" color={color} size={size} /> }} />
      <Tabs.Screen name="profile" options={{ title: "Profile", tabBarButtonTestID: "Tab Profile", tabBarIcon: ({ color, size }) => <Ionicons name="person-outline" color={color} size={size} /> }} />
    </Tabs>
  );
}
