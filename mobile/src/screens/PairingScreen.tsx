import React, { useState } from "react";
import { ActivityIndicator, StyleSheet, Text, TextInput, View } from "react-native";
import { pairDevice } from "../api/pairing";
import { PrimaryButton } from "../components/PrimaryButton";
import { palette, radii, spacing } from "../theme";

export function PairingScreen({ onPaired }: { onPaired: (token: string, userId: string) => void }) {
  const [code, setCode] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handlePair = async () => {
    setLoading(true);
    setError(null);
    const result = await pairDevice(code.trim());
    if (!result.ok || !result.data) {
      setError(result.error || "Pairing failed");
      setLoading(false);
      return;
    }
    onPaired(result.data.access_token, result.data.user_id);
    setLoading(false);
  };

  return (
    <View style={styles.container}>
      <Text style={styles.title}>Pair Your Device</Text>
      <Text style={styles.subtitle}>Enter the 8‑character pairing code from the admin console.</Text>
      <TextInput
        style={styles.input}
        placeholder="PAIRING CODE"
        placeholderTextColor={palette.steel}
        autoCapitalize="characters"
        value={code}
        onChangeText={setCode}
      />
      {error ? <Text style={styles.error}>{error}</Text> : null}
      {loading ? (
        <ActivityIndicator color={palette.aqua} />
      ) : (
        <PrimaryButton label="Connect" onPress={handlePair} disabled={!code.trim()} />
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    padding: spacing.xl,
    justifyContent: "center",
    backgroundColor: palette.midnight,
  },
  title: {
    fontSize: 28,
    fontWeight: "700",
    color: palette.mist,
    marginBottom: spacing.sm,
  },
  subtitle: {
    color: palette.steel,
    marginBottom: spacing.lg,
    fontSize: 15,
  },
  input: {
    backgroundColor: palette.slate,
    borderRadius: radii.md,
    padding: spacing.md,
    color: palette.mist,
    marginBottom: spacing.md,
    letterSpacing: 1.2,
  },
  error: {
    color: palette.rose,
    marginBottom: spacing.sm,
  },
});
