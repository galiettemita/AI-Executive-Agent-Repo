import React from "react";
import { Pressable, StyleSheet, Text } from "react-native";
import { palette, radii, spacing } from "../theme";

export function PrimaryButton({
  label,
  onPress,
  disabled,
}: {
  label: string;
  onPress: () => void;
  disabled?: boolean;
}) {
  return (
    <Pressable
      onPress={onPress}
      style={[styles.button, disabled && styles.disabled]}
      disabled={disabled}
    >
      <Text style={styles.label}>{label}</Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  button: {
    backgroundColor: palette.aqua,
    paddingVertical: spacing.md,
    borderRadius: radii.md,
    alignItems: "center",
  },
  disabled: {
    opacity: 0.6,
  },
  label: {
    color: palette.midnight,
    fontSize: 16,
    fontWeight: "700",
    letterSpacing: 0.4,
  },
});
