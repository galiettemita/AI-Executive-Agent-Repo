import React from "react";
import { StyleSheet, Text, View } from "react-native";
import { LinearGradient } from "expo-linear-gradient";
import { PrimaryButton } from "../components/PrimaryButton";
import { API_BASE_URL } from "../api/client";
import { palette, radii, spacing } from "../theme";

export function SettingsScreen({
  userId,
  onSignOut,
}: {
  userId: string;
  onSignOut: () => void;
}) {
  return (
    <LinearGradient colors={[palette.midnight, palette.ocean]} style={styles.container}>
      <Text style={styles.title}>Settings</Text>
      <View style={styles.card}>
        <Text style={styles.label}>User ID</Text>
        <Text style={styles.value}>{userId}</Text>
        <Text style={styles.label}>API Base URL</Text>
        <Text style={styles.value}>{API_BASE_URL}</Text>
        <PrimaryButton label="Sign out" onPress={onSignOut} />
      </View>
    </LinearGradient>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    padding: spacing.xl,
  },
  title: {
    fontSize: 26,
    fontWeight: "700",
    color: palette.mist,
    marginBottom: spacing.lg,
  },
  card: {
    backgroundColor: palette.slate,
    borderRadius: radii.lg,
    padding: spacing.lg,
    gap: spacing.sm,
  },
  label: {
    color: palette.steel,
    fontSize: 12,
    textTransform: "uppercase",
    letterSpacing: 1.2,
  },
  value: {
    color: palette.mist,
    marginBottom: spacing.sm,
  },
});
