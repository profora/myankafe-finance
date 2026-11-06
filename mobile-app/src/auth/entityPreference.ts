import AsyncStorage from "@react-native-async-storage/async-storage";

const KEY = "mk_finance_entity_public_id";

export async function saveEntityPreference(publicId: string) {
  await AsyncStorage.setItem(KEY, publicId);
}

export async function loadEntityPreference(): Promise<string | null> {
  return AsyncStorage.getItem(KEY);
}
