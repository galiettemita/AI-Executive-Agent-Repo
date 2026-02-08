import React from "react";
import { FlatList, StyleSheet, Text, View } from "react-native";
import { LinearGradient } from "expo-linear-gradient";
import { fetchEmailAlerts, fetchNotifications } from "../api/alerts";
import { palette, radii, spacing } from "../theme";

export function AlertsScreen({ userId, token }: { userId: string; token: string }) {
  const [items, setItems] = React.useState<any[]>([]);
  const [error, setError] = React.useState<string | null>(null);

  const load = async () => {
    setError(null);
    const [alerts, notifications] = await Promise.all([
      fetchEmailAlerts(userId, token),
      fetchNotifications(userId, token),
    ]);

    if (!alerts.ok && !notifications.ok) {
      setError(alerts.error || notifications.error || "Failed to load alerts");
      return;
    }

    const merged = [
      ...(alerts.data?.alerts || []).map((a: any) => ({
        type: "email",
        title: a.subject || "Email alert",
        subtitle: a.sender,
        detail: a.reason,
        created_at: a.created_at,
      })),
      ...(notifications.data?.items || []).map((n: any) => ({
        type: "notification",
        title: n.title,
        subtitle: n.message,
        detail: n.event_type,
        created_at: n.created_at,
      })),
    ];

    setItems(merged);
  };

  React.useEffect(() => {
    load();
  }, []);

  return (
    <LinearGradient colors={[palette.midnight, palette.ocean]} style={styles.container}>
      <Text style={styles.title}>Alerts</Text>
      {error ? <Text style={styles.error}>{error}</Text> : null}
      <FlatList
        data={items}
        keyExtractor={(_, idx) => `${idx}`}
        contentContainerStyle={{ gap: spacing.md, paddingBottom: spacing.xl }}
        renderItem={({ item }) => (
          <View style={styles.card}>
            <Text style={styles.cardTitle}>{item.title}</Text>
            {item.subtitle ? <Text style={styles.cardSubtitle}>{item.subtitle}</Text> : null}
            {item.detail ? <Text style={styles.cardDetail}>{item.detail}</Text> : null}
          </View>
        )}
      />
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
  error: {
    color: palette.rose,
    marginBottom: spacing.md,
  },
  card: {
    backgroundColor: palette.slate,
    borderRadius: radii.md,
    padding: spacing.md,
  },
  cardTitle: {
    color: palette.mist,
    fontWeight: "700",
  },
  cardSubtitle: {
    color: palette.citrus,
    marginTop: spacing.xs,
  },
  cardDetail: {
    color: palette.steel,
    marginTop: spacing.xs,
  },
});
