import { useState } from "react";
import { Modal, Pressable, StyleSheet, Text, View } from "react-native";
import { useEntity } from "../auth/EntityContext";
import { colors } from "../theme";

export function EntitySwitcher() {
  const { entity, entities, select } = useEntity();
  const [open, setOpen] = useState(false);
  if (!entity) return null;
  return (
    <>
      <Pressable
        accessibilityRole="button"
        accessibilityLabel={`Current entity ${entity.Name}`}
        onPress={() => setOpen(true)}
        style={styles.chip}
      >
        <Text style={styles.name}>{entity.Name}</Text>
        <Text style={styles.meta}>
          {entity.Role} · {entity.FunctionalCurrency}
        </Text>
      </Pressable>
      <Modal visible={open} animationType="slide" transparent>
        <View style={styles.backdrop}>
          <View style={styles.sheet}>
            <Text style={styles.title}>Switch entity</Text>
            {entities.map((item) => (
              <Pressable
                key={item.PublicID}
                accessibilityRole="button"
                accessibilityLabel={`Switch to ${item.Name}`}
                style={styles.row}
                onPress={() => {
                  void select(item.PublicID);
                  setOpen(false);
                }}
              >
                <Text style={styles.name}>{item.Name}</Text>
                <Text style={styles.meta}>
                  {item.Role} · {item.FunctionalCurrency}
                </Text>
              </Pressable>
            ))}
            <Pressable accessibilityRole="button" accessibilityLabel="Close entity switcher" onPress={() => setOpen(false)} style={styles.close}>
              <Text style={styles.closeText}>Close</Text>
            </Pressable>
          </View>
        </View>
      </Modal>
    </>
  );
}

const styles = StyleSheet.create({
  chip: { paddingVertical: 8, minHeight: 44, justifyContent: "center" },
  name: { color: colors.ink, fontSize: 18, fontWeight: "800" },
  meta: { color: colors.muted },
  backdrop: { flex: 1, justifyContent: "flex-end", backgroundColor: "rgba(11,31,51,0.4)" },
  sheet: { backgroundColor: colors.paper, padding: 16, gap: 8, borderTopLeftRadius: 16, borderTopRightRadius: 16 },
  title: { fontSize: 18, fontWeight: "800", color: colors.ink },
  row: { minHeight: 48, justifyContent: "center" },
  close: { minHeight: 48, justifyContent: "center" },
  closeText: { color: colors.accent, fontWeight: "700" },
});
