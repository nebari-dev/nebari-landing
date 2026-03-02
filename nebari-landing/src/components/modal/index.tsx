import { useRef, type ReactNode } from "react";
import {
  Modal,
  ModalRef,
  ModalHeading,
  ModalFooter,
  ModalToggleButton,
  ModalCloseButton,
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

        <ModalCloseButton handleClose={() => modalRef.current?.toggleModal(undefined, false)} />
      </Modal>
    </>
  );
}
