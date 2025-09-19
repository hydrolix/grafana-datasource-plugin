import React, { createContext, useContext, useMemo } from "react";
import {
  AssistantConfig,
  AssistantConversation,
  AssistantConversationStateActions,
  AssistantConversationStateStore,
  AssistantMessage,
  AssistantConversationState,
  createAssistantConversation,
  createAssistantConversationStateStore,
} from "../../assistant/assistantConversation";
import { useStoreWithEqualityFn } from "zustand/traditional";

interface AssistantConversationContextValue {
  conversation: AssistantConversation;
  stateStore: AssistantConversationStateStore;
}

const AssistantConversationContext =
  createContext<AssistantConversationContextValue | null>(null);

export const createAssistantConversationContextValue = (
  config: AssistantConfig,
  messages: AssistantMessage[] = []
): AssistantConversationContextValue => {
  const stateStore = createAssistantConversationStateStore(messages);
  const conversation = createAssistantConversation(config, stateStore);
  return { conversation: conversation, stateStore: stateStore };
};

type AssistantConversationProviderProps = React.PropsWithChildren<{
  config: AssistantConfig;
}>;

export const AssistantConversationProvider: React.FC<
  AssistantConversationProviderProps
> = ({ config, children }) => {
  const value = useMemo(
    () => createAssistantConversationContextValue(config, []),
    [config]
  );

  return (
    <AssistantConversationContext.Provider value={value}>
      {children}
    </AssistantConversationContext.Provider>
  );
};

export function useAssistantConversation(): AssistantConversation {
  const context = useContext(AssistantConversationContext);
  if (!context) {
    throw new Error(
      "useAssistantConversation must be used within AssistantProvider"
    );
  }
  return context.conversation;
}

export function useAssistantConversationState<T>(
  selector: (state: AssistantConversationState) => T
): T {
  const context = useContext(AssistantConversationContext);
  if (!context) {
    throw new Error(
      "useAssistantConversationState must be used within AssistantProvider"
    );
  }
  return useStoreWithEqualityFn<AssistantConversationStateStore, T>(
    context.stateStore,
    selector
  );
}

export function useAssistantConversationStateActions<T>(
  selector: (actions: AssistantConversationStateActions) => T
): T {
  const context = useContext(AssistantConversationContext);
  if (!context) {
    throw new Error(
      "useAssistantConversationStateActions must be used within AssistantProvider"
    );
  }
  return useStoreWithEqualityFn<AssistantConversationStateStore, T>(
    context.stateStore,
    selector
  );
}
