import { useNetworkState } from "expo-network";
import { StyleSheet, Text, View } from "react-native";
import { colors } from "../theme";

export function useOffline() {
  const state = useNetworkState();
  return state.isConnected === false || state.isInternetReachable === false;
}

export function NetworkBanner() {
  const offline = useOffline();
  if (!offline) return null;
  return (
    <View style={styles.banner}>
      <Text style={styles.text}>Offline. Accounting changes stay disabled until you are online.</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  banner: { backgroundColor: "#8A5A00", padding: 10 },
  text: { color: "#fff", fontWeight: "700" },
});
