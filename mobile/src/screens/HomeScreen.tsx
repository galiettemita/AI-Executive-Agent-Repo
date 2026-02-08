import React from "react";
import { StyleSheet, Text, View } from "react-native";
import { LinearGradient } from "expo-linear-gradient";
import { PrimaryButton } from "../components/PrimaryButton";
import { palette, radii, spacing } from "../theme";
import { runEmailMonitoring } from "../api/monitoring";

export function HomeScreen({ userId, token }: { userId: string; token: string }) {
  const [status, setStatus] = React.useState<string>("Idle");

  const handleRun = async () => {
    setStatus("Running...");
    const result = await runEmailMonitoring(userId, token);
    if (!result.ok) {
      setStatus(result.error || "Run failed");
      return;
    }
    const alerts = result.data?.result?.alerts ?? 0;
    setStatus(`Completed · ${alerts} alerts`);
  };

  return (
    <LinearGradient colors={[palette.ocean, palette.midnight]} style={styles.container}>
      <View style={styles.header}>
        <Text style={styles.title}>Executive AI Agent</Text>
        <Text style={styles.subtitle}>Companion app foundation</Text>
      </View>
      <View style={styles.card}>
        <Text style={styles.cardTitle}>Email monitoring</Text>
        <Text style={styles.cardBody}>Trigger a manual run and review alerts.</Text>
        <PrimaryButton label="Run monitoring" onPress={handleRun} />
        <Text style={styles.status}>{status}</Text>
      </View>
    </LinearGradient>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    padding: spacing.xl,
  },
  header: {
    marginTop: spacing.lg,
    marginBottom: spacing.lg,
  },
  title: {
    fontSize: 28,
    fontWeight: "700",
    color: palette.mist,
  },
  subtitle: {
    marginTop: spacing.xs,
    color: palette.steel,
  },
  card: {
    backgroundColor: palette.slate,
    borderRadius: radii.lg,
    padding: spacing.lg,
    gap: spacing.md,
  },
  cardTitle: {
    fontSize: 18,
    fontWeight: "700",
    color: palette.mist,
  },
  cardBody: {
    color: palette.steel,
  },
  status: {
    color: palette.citrus,
  },
});
