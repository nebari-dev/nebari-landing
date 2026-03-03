import { useRef, type ReactNode } from "react";
import type { ModalRef } from "@trussworks/react-uswds";
import {
  Modal,
  ModalHeading,
  ModalFooter,
  ModalToggleButton,
} from "@trussworks/react-uswds";

export default function AppModal(): ReactNode {
  const modalRef = useRef<ModalRef>(null);

  return (
    <>
      <ModalToggleButton modalRef={modalRef} opener>
        Open modal
      </ModalToggleButton>

      <Modal id="example-modal" ref={modalRef}>
        <ModalHeading>Simple modal</ModalHeading>

        <p>This is the simplest modal setup.</p>

        <ModalFooter>
          <ModalToggleButton modalRef={modalRef} closer unstyled>
            Close
          </ModalToggleButton>
        </ModalFooter>
      </Modal>
    </>
  );
}
