import { ReactNode, useState } from "react";
import { ActivityIndicator, Modal, Pressable, ScrollView, StyleSheet, Text, TextInput, View } from "react-native";
import { colors, space } from "../theme";

export function Screen({ children }: { children: ReactNode }) {
  return <View style={styles.screen}>{children}</View>;
}

export function Card({ children }: { children: ReactNode }) {
  return <View style={styles.card}>{children}</View>;
}

export function Button({
  label,
  onPress,
  disabled,
  tone = "primary",
}: {
  label: string;
  onPress: () => void;
  disabled?: boolean;
  tone?: "primary" | "secondary" | "danger";
}) {
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={label}
      testID={label}
      accessibilityState={{ disabled: Boolean(disabled) }}
      onPress={onPress}
      disabled={disabled}
      style={[styles.button, tone === "secondary" && styles.secondary, tone === "danger" && styles.danger, disabled && styles.disabled]}
    >
      <Text style={[styles.buttonText, tone === "secondary" && styles.secondaryText]}>{label}</Text>
    </Pressable>
  );
}

export function TextField({
  label,
  value,
  onChangeText,
  secure,
  placeholder,
  keyboardType,
}: {
  label: string;
  value: string;
  onChangeText: (v: string) => void;
  secure?: boolean;
  placeholder?: string;
  keyboardType?: "default" | "decimal-pad";
}) {
  return (
    <View style={styles.field}>
      <Text style={styles.label}>{label}</Text>
      <TextInput
        value={value}
        onChangeText={onChangeText}
        secureTextEntry={secure}
        placeholder={placeholder}
        keyboardType={keyboardType}
        testID={label}
        accessibilityLabel={label}
        autoCapitalize="none"
        style={styles.input}
      />
    </View>
  );
}

export function SelectField({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string;
  options: { id: string; label: string }[];
  onChange: (id: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const selected = options.find((option) => option.id === value);
  return (
    <View style={styles.field}>
      <Text style={styles.label}>{label}</Text>
      <Pressable
        accessibilityRole="button"
        accessibilityLabel={label}
        testID={label}
        onPress={() => setOpen(true)}
        style={styles.input}
      >
        <Text style={selected ? styles.optionText : styles.muted}>{selected?.label || "Choose"}</Text>
      </Pressable>
      <Modal visible={open} transparent animationType="slide" onRequestClose={() => setOpen(false)}>
        <View style={styles.backdrop}>
          <Pressable
            accessibilityRole="button"
            accessibilityLabel={`Dismiss ${label}`}
            testID={`Dismiss ${label}`}
            style={StyleSheet.absoluteFillObject}
            onPress={() => setOpen(false)}
          />
          <View style={styles.sheet}>
            <Text style={styles.sheetTitle}>{label}</Text>
            <ScrollView keyboardShouldPersistTaps="handled">
              {options.map((option) => (
                <Pressable
                  key={`${option.id}:${option.label}`}
                  accessibilityRole="button"
                  accessibilityLabel={option.label}
                  testID={`${label} option ${option.label}`}
                  accessibilityState={{ selected: option.id === value }}
                  onPress={() => {
                    onChange(option.id);
                    setOpen(false);
                  }}
                  style={styles.sheetRow}
                >
                  <Text style={styles.optionText}>{option.label}</Text>
                </Pressable>
              ))}
            </ScrollView>
            <Pressable accessibilityRole="button" accessibilityLabel="Close" testID={`Close ${label}`} onPress={() => setOpen(false)} style={styles.sheetRow}>
              <Text style={styles.closeText}>Close</Text>
            </Pressable>
          </View>
        </View>
      </Modal>
    </View>
  );
}

export function StatusBadge({ status }: { status: string }) {
  const label = status === "DRAFT" ? "Draft" : status === "POSTED" ? "Posted" : status === "VOIDED" ? "Voided" : status;
  return (
    <View style={styles.badge} accessibilityRole="text" accessibilityLabel={`Status ${label}`} testID={`Status ${label}`}>
      <Text style={styles.badgeText}>{label}</Text>
    </View>
  );
}

export function Page({ children }: { children: ReactNode }) {
  return <View style={styles.page}>{children}</View>;
}

export function LoadingState({ label = "Loading" }: { label?: string }) {
  return (
    <View style={styles.center}>
      <ActivityIndicator color={colors.accent} />
      <Text style={styles.muted}>{label}</Text>
    </View>
  );
}

export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <View style={styles.center}>
      <Text style={styles.error}>{message}</Text>
      {onRetry ? <Button label="Try again" onPress={onRetry} tone="secondary" /> : null}
    </View>
  );
}

export function EmptyState({ message }: { message: string }) {
  return (
    <View style={styles.center}>
      <Text style={styles.muted}>{message}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1, backgroundColor: colors.paper },
  card: { backgroundColor: colors.card, borderRadius: 14, padding: space.md, borderWidth: 1, borderColor: colors.line, gap: 6 },
  button: { minHeight: 48, borderRadius: 12, backgroundColor: colors.accent, alignItems: "center", justifyContent: "center", paddingHorizontal: 16 },
  secondary: { backgroundColor: colors.card, borderWidth: 1, borderColor: colors.line },
  danger: { backgroundColor: colors.negative },
  disabled: { opacity: 0.45 },
  buttonText: { color: colors.accentInk, fontWeight: "700", fontSize: 16 },
  secondaryText: { color: colors.ink },
  field: { gap: 6 },
  label: { color: colors.inkSoft, fontWeight: "600" },
  input: { minHeight: 48, borderWidth: 1, borderColor: colors.line, borderRadius: 12, paddingHorizontal: 12, backgroundColor: colors.card, fontSize: 16, color: colors.ink, justifyContent: "center" },
  page: { width: "100%", maxWidth: 840, alignSelf: "center", gap: 12 },
  backdrop: { flex: 1, justifyContent: "flex-end", backgroundColor: "rgba(11,31,51,0.45)" },
  sheet: { maxHeight: "80%", backgroundColor: colors.paper, borderTopLeftRadius: 16, borderTopRightRadius: 16, padding: space.md, gap: 8 },
  sheetTitle: { fontSize: 18, fontWeight: "800", color: colors.ink },
  sheetRow: { minHeight: 48, justifyContent: "center" },
  closeText: { color: colors.accent, fontWeight: "700" },
  options: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  option: { minHeight: 44, justifyContent: "center", paddingHorizontal: 12, borderRadius: 12, borderWidth: 1, borderColor: colors.line, backgroundColor: colors.card },
  optionOn: { backgroundColor: colors.ink, borderColor: colors.ink },
  optionText: { color: colors.ink },
  optionOnText: { color: colors.accentInk, fontWeight: "700" },
  badge: { alignSelf: "flex-start", borderWidth: 1, borderColor: colors.ink, borderRadius: 999, paddingHorizontal: 8, paddingVertical: 2 },
  badgeText: { color: colors.ink, fontSize: 12, fontWeight: "700" },
  center: { padding: space.lg, gap: 12, alignItems: "flex-start" },
  muted: { color: colors.muted, fontSize: 16 },
  error: { color: colors.negative, fontSize: 16 },
});
