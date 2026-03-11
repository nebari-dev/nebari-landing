import type{
  ReactNode
} from "react";

import {
  Card, CardBody
} from "@trussworks/react-uswds";

import {
  StatusTag
} from "../statustag"

import
  AvatarImage
from "../icon";

import "./index.scss";

export function SimpleCard(props: SimpleCardProps): ReactNode {
  
  // Extract the props
  const { image, name, status, url } = props;

  return (
    <Card className="app-card app-card--simple" layout="standardDefault">
      <CardBody className="app-card__body app-card__body--simple">
        <a
          href={url}
          target="_blank"
          rel="noopener noreferrer"
          className="app-card-simple__link"
          aria-label={`${name} (opens in a new tab)`}
        >
          <div className="app-card-simple__row">
            <div className="app-card-simple__avatar">
              {AvatarImage(image)}
            </div>

            <div className="app-card-simple__meta">
              <h3 className="app-card__title app-card__title--simple">{name}</h3>

              <StatusTag status={status} />
            </div>
          </div>
        </a>
      </CardBody>
    </Card>
  );
}

export type SimpleCardProps = {
  image: string;
  name: string;
  status: string;
  url: string;
};
