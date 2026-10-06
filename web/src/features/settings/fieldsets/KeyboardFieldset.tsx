import { CheckboxField, Fieldset, type Register } from './Field.tsx'

// KeyboardFieldset is whether a single key, pressed outside a text field, does
// what the terminal's does. It is off until turned on, since a key that acts
// on its own surprises a screen reader or speech user; ? and the palette work
// either way, and ui.keys, in the file, moves a key.
export function KeyboardFieldset({ register }: { register: Register }) {
  return (
    <Fieldset
      legend="Keyboard"
      hint="A key pressed outside a text field does what it does in the terminal, as ui.keys sets it. The ? sheet of keys and the palette, on Ctrl+K or ⌘K, work whether this is on or off."
    >
      <CheckboxField register={register} name="ui.web_shortcuts" label="Single-key shortcuts" />
    </Fieldset>
  )
}
